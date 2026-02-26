/** HTTP download/upload test execution. */

import { getApiBase, state, TEST_CONFIG } from "./state.js";
import {
  sleep,
  retryAfterMs,
  isNetworkError,
  fetchWithTimeout,
} from "./utils.js";
import { createWarmUpDetector, createEarlyStopDetector } from "./warmup.js";
import { createDiagnosticsCollector } from "./diagnostics.js";

function resolveStreamsInner() {
  if (!state.settings.streams || Number.isNaN(state.settings.streams)) {
    return 4;
  }
  return state.settings.streams;
}

function resolveChunkSize() {
  return 1024 * 1024;
}

function detectOverheadFactor() {
  try {
    const entries = performance.getEntriesByType("resource");
    for (let i = entries.length - 1; i >= 0; i--) {
      const e = entries[i];
      if (e.name && e.name.includes("/api/v1/") && e.nextHopProtocol) {
        if (e.nextHopProtocol === "h2" || e.nextHopProtocol === "h3") return 1;
        return 1.02;
      }
    }
  } catch (err) {
    console.debug("protocol detection failed", err);
  }
  return 1.02;
}

function buildDownloadChunkAttempts(chunkSize) {
  const preferredFallback = 256 * 1024;
  const attempts = [chunkSize];
  if (preferredFallback < chunkSize) attempts.push(preferredFallback);
  if (65536 < (attempts.at(-1) ?? 0)) attempts.push(65536);
  return attempts;
}

function classifyDownloadStreamError(e, signal, streamState) {
  if (e.name === "AbortError" || signal.aborted) return "aborted";
  if (e.status === 503 || e.status === 429) {
    streamState.sawOverload = true;
    return "overloaded";
  }
  if (isNetworkError(e)) {
    streamState.sawNetworkError = true;
    return "network_retry";
  }
  return "failed";
}

async function handleDownloadStreamCatch(e, options) {
  const {
    attemptIndex,
    attemptsLength,
    signal,
    maxNetworkRetries,
    retryDelayMs,
    streamState,
  } = options;
  const action = classifyDownloadStreamError(e, signal, streamState);
  if (action === "aborted") return { done: true, result: "aborted" };
  if (action === "overloaded") {
    await sleep(e.retryAfter || 500);
    return { done: true, result: "overloaded" };
  }
  if (action === "failed") {
    const msg =
      attemptIndex < attemptsLength - 1
        ? "Download stream failed, retrying smaller chunk"
        : "Download stream failed after retries";
    console.warn(msg, e);
    return { done: true, result: "failed" };
  }
  if (action === "network_retry") return { done: false, retry: true };
  return { done: true, result: "failed" };
}

async function tryDownloadChunkWithRetries(options) {
  const {
    attemptChunk,
    attemptIndex,
    attemptsLength,
    downloadStream,
    signal,
    maxNetworkRetries,
    retryDelayMs,
    streamState,
  } = options;
  for (let retry = 0; retry <= maxNetworkRetries; retry++) {
    if (signal.aborted) return "aborted";
    try {
      if (await downloadStream(attemptChunk)) return "success";
      return "failed";
    } catch (e) {
      const outcome = await handleDownloadStreamCatch(e, {
        attemptIndex,
        attemptsLength,
        signal,
        maxNetworkRetries,
        retryDelayMs,
        streamState,
      });
      if (outcome.done) return outcome.result;
      await sleep(retryDelayMs);
    }
  }
  return "failed";
}

async function executeDownloadAttempts(
  attempts,
  downloadStream,
  signal,
  maxNetworkRetries,
  retryDelayMs,
  streamState,
) {
  for (let attemptIndex = 0; attemptIndex < attempts.length; attemptIndex++) {
    if (signal.aborted) return "aborted";
    const result = await tryDownloadChunkWithRetries({
      attemptChunk: attempts[attemptIndex],
      attemptIndex,
      attemptsLength: attempts.length,
      downloadStream,
      signal,
      maxNetworkRetries,
      retryDelayMs,
      streamState,
    });
    if (
      result === "success" ||
      result === "aborted" ||
      result === "overloaded"
    ) {
      return result;
    }
  }
  return "failed";
}

function processDownloadChunk(value, now, ctx) {
  const s = ctx.readState;
  const measuring = ctx.warmUp.settled();
  s.allBytes += value.length;
  if (measuring) {
    s.totalBytes += value.length;
  } else {
    ctx.warmUp.record(value.length, now);
    if (ctx.warmUp.settled()) {
      s.totalBytes = 0;
      s.measureStartTime = now;
    }
  }
  if (ctx.diagnostics) ctx.diagnostics.record(value.length, now, measuring);
  if (ctx.earlyStop && measuring && ctx.earlyStop.record(value.length, now)) {
    ctx.endTimeRef.value = now;
  }
  const elapsedSec = (now - ctx.startTime) / 1000;
  const phaseDurationMs = Math.max(1, ctx.endTimeRef.value - ctx.startTime);
  const phaseProgress = Math.min(
    100,
    Math.max(0, ((now - ctx.startTime) / phaseDurationMs) * 100),
  );
  const displayBytes = measuring ? s.totalBytes : s.allBytes;
  ctx.onProgress(displayBytes, elapsedSec, phaseProgress);
}

export async function runDownloadTest(duration, onProgress, signal) {
  const startTime = performance.now();
  const numStreams = resolveStreamsInner();
  const chunkSize = resolveChunkSize();
  const streamDelay = TEST_CONFIG.STREAM_DELAY_MS;
  const maxNetworkRetries = TEST_CONFIG.MAX_NETWORK_RETRIES;
  const retryDelayMs = TEST_CONFIG.NETWORK_RETRY_DELAY_MS;
  const nominalEndTime = startTime + duration * 1000;
  const endTimeRef = { value: nominalEndTime };
  const streamState = {
    sawNetworkError: false,
    sawOverload: false,
    successfulStreams: 0,
  };

  const warmUp = createWarmUpDetector(duration * 1000);
  const earlyStop = createEarlyStopDetector(() => warmUp.settled());
  const diagnostics = createDiagnosticsCollector(TEST_CONFIG.WARMUP_WINDOW_MS);
  const readState = { totalBytes: 0, measureStartTime: 0, allBytes: 0 };

  const downloadStream = async (chunk) => {
    const res = await fetchWithTimeout(
      `${getApiBase()}/download?duration=${duration}&chunk=${chunk}`,
      {
        method: "GET",
        cache: "no-store",
        credentials: "omit",
        signal,
      },
      duration * 1000 + TEST_CONFIG.HTTP_TIMEOUT_BUFFER_MS,
    );

    if (!res.ok || !res.body) {
      await res.text().catch(() => {});
      if (res.status === 503 || res.status === 429) {
        const err = new Error("Server overloaded");
        err.status = res.status;
        err.retryAfter = retryAfterMs(res, 500);
        throw err;
      }
      return false;
    }

    const reader = res.body.getReader();
    const readCtx = {
      warmUp,
      readState,
      startTime,
      endTimeRef,
      earlyStop,
      diagnostics,
      onProgress,
    };
    try {
      while (true) {
        if (signal.aborted) break;
        const now = performance.now();
        if (now >= endTimeRef.value) {
          await reader.cancel();
          break;
        }
        const { done, value } = await reader.read();
        if (done) break;
        processDownloadChunk(value, now, readCtx);
      }
    } finally {
      reader.releaseLock();
    }
    return true;
  };

  const streamPromises = [];
  for (let i = 0; i < numStreams; i++) {
    const delay = i * streamDelay;
    const streamPromise = (async () => {
      await new Promise((r) => setTimeout(r, delay));
      const attempts = buildDownloadChunkAttempts(chunkSize);
      const result = await executeDownloadAttempts(
        attempts,
        downloadStream,
        signal,
        maxNetworkRetries,
        retryDelayMs,
        streamState,
      );
      if (result === "success") streamState.successfulStreams += 1;
    })();
    streamPromises.push(streamPromise);
  }

  await Promise.all(streamPromises);

  const overheadFactor = detectOverheadFactor();
  const endNow = Math.min(performance.now(), endTimeRef.value);
  const { totalBytes } = readState;
  const actualMeasureStart =
    readState.measureStartTime > 0 ? readState.measureStartTime : startTime;
  const measureTime = Math.max(
    TEST_CONFIG.MIN_MEASURE_SECONDS,
    (endNow - actualMeasureStart) / 1000,
  );
  const avgSpeed = (totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;

  throwIfZeroBytes(streamState, totalBytes, {
    network: "Network error during download. Try again or change server.",
    overload: "Server overloaded. Try again in a moment or change server.",
    noStreams: "Download failed. No stream completed successfully.",
  });

  const stopReason = signal.aborted
    ? "aborted"
    : endTimeRef.value < nominalEndTime - 500
      ? "early_stable"
      : "duration";
  const diag = diagnostics.finish(stopReason);
  state.diagnostics = state.diagnostics || {};
  state.diagnostics.download = diag;

  return Math.max(avgSpeed, 0);
}

function throwIfZeroBytes(state, totalBytes, messages) {
  if (totalBytes > 0) return;
  if (state.sawNetworkError) throw new Error(messages.network);
  if (state.sawOverload) throw new Error(messages.overload);
  if (state.successfulStreams === 0) throw new Error(messages.noStreams);
}

async function sendUploadRequest(blob, duration, signal) {
  return fetchWithTimeout(
    `${getApiBase()}/upload`,
    {
      method: "POST",
      body: blob,
      headers: { "Content-Type": "application/octet-stream" },
      cache: "no-store",
      credentials: "omit",
      signal,
    },
    duration * 1000 + TEST_CONFIG.HTTP_TIMEOUT_BUFFER_MS,
  );
}

function recordUploadProgress(
  metricsState,
  warmUp,
  blobSize,
  now,
  startTime,
  onProgress,
  extra,
) {
  const measuring = warmUp.settled();
  metricsState.allBytes += blobSize;
  metricsState.successfulStreams += 1;

  if (measuring) {
    metricsState.totalBytes += blobSize;
  } else {
    warmUp.record(blobSize, now);
    if (warmUp.settled()) {
      metricsState.totalBytes = 0;
      metricsState.measureStartTime = now;
    }
  }

  if (extra?.diagnostics) extra.diagnostics.record(blobSize, now, measuring);
  if (extra?.earlyStop && measuring && extra.earlyStop.record(blobSize, now)) {
    extra.endTimeRef.value = now;
  }

  const elapsedSec = (now - startTime) / 1000;
  const phaseDurationMs = Math.max(1, extra.endTimeRef.value - startTime);
  const phaseProgress = Math.min(
    100,
    Math.max(0, ((now - startTime) / phaseDurationMs) * 100),
  );
  const displayBytes = measuring
    ? metricsState.totalBytes
    : metricsState.allBytes;
  onProgress(displayBytes, elapsedSec, phaseProgress);
}

function handleUploadNonOkResponse(res, metricsState, retryAfterMs) {
  if (res.status === 503 || res.status === 429) {
    metricsState.sawOverload = true;
    return { action: "overload_break", retryAfter: retryAfterMs(res, 500) };
  }
  return { action: "retry_or_break" };
}

function handleUploadCatchError(
  e,
  metricsState,
  consecutiveErrors,
  maxRetries,
) {
  if (e.name === "AbortError") return { action: "break" };
  if (isNetworkError(e)) {
    metricsState.sawNetworkError = true;
    const next = consecutiveErrors + 1;
    return next <= maxRetries ? { action: "retry", next } : { action: "throw" };
  }
  return { action: "throw" };
}

async function processUploadResponse(res, options) {
  const {
    blob,
    metricsState,
    retryAfterMs,
    maxNetworkRetries,
    retryDelayMs,
    warmUp,
    startTime,
    onProgress,
    extra,
  } = options;
  if (res.ok) {
    await res.text().catch(() => {});
    const now = performance.now();
    recordUploadProgress(
      metricsState,
      warmUp,
      blob.size,
      now,
      startTime,
      onProgress,
      extra,
    );
    return { ok: true };
  }
  await res.text().catch(() => {});
  const hr = handleUploadNonOkResponse(res, metricsState, retryAfterMs);
  if (hr.action === "overload_break") {
    await sleep(hr.retryAfter);
    return { ok: false, overload: true };
  }
  return { ok: false, retry: true };
}

async function runSingleUploadStream(options) {
  const {
    delay,
    blob,
    endTimeRef,
    duration,
    signal,
    maxNetworkRetries,
    retryDelayMs,
    warmUp,
    metricsState,
    startTime,
    onProgress,
    extra,
  } = options;
  await new Promise((r) => setTimeout(r, delay));

  let consecutiveErrors = 0;
  while (performance.now() < endTimeRef.value && !signal.aborted) {
    try {
      const res = await sendUploadRequest(blob, duration, signal);
      const pr = await processUploadResponse(res, {
        blob,
        metricsState,
        retryAfterMs,
        maxNetworkRetries,
        retryDelayMs,
        warmUp,
        startTime,
        onProgress,
        extra,
      });
      if (pr.ok) {
        consecutiveErrors = 0;
        continue;
      }
      if (pr.overload) break;
      consecutiveErrors += 1;
      if (consecutiveErrors > maxNetworkRetries) break;
      await sleep(retryDelayMs);
    } catch (e) {
      const hc = handleUploadCatchError(
        e,
        metricsState,
        consecutiveErrors,
        maxNetworkRetries,
      );
      if (hc.action === "break") break;
      if (hc.action === "retry") {
        consecutiveErrors = hc.next;
        await sleep(retryDelayMs);
        continue;
      }
      throw e;
    }
  }
}

export async function runUploadTest(duration, onProgress, signal) {
  const startTime = performance.now();
  const numStreams = resolveStreamsInner();
  const chunkSize = resolveChunkSize();
  const streamDelay = TEST_CONFIG.STREAM_DELAY_MS;
  const blobSize = chunkSize;
  const maxNetworkRetries = TEST_CONFIG.MAX_NETWORK_RETRIES;
  const retryDelayMs = TEST_CONFIG.NETWORK_RETRY_DELAY_MS;
  const metricsState = {
    totalBytes: 0,
    allBytes: 0,
    sawNetworkError: false,
    sawOverload: false,
    successfulStreams: 0,
    measureStartTime: 0,
  };

  const warmUp = createWarmUpDetector(duration * 1000);
  const earlyStop = createEarlyStopDetector(() => warmUp.settled());
  const diagnostics = createDiagnosticsCollector(TEST_CONFIG.WARMUP_WINDOW_MS);
  const nominalEndTime = startTime + duration * 1000;
  const endTimeRef = { value: nominalEndTime };
  const extra = { earlyStop, diagnostics, endTimeRef };

  const chunks = [];
  for (let i = 0; i < blobSize; i += TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES) {
    const piece = new Uint8Array(
      Math.min(TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES, blobSize - i),
    );
    crypto.getRandomValues(piece);
    chunks.push(piece);
  }
  const blob = new Blob(chunks);

  const streams = [];
  for (let i = 0; i < numStreams; i++) {
    streams.push(
      runSingleUploadStream({
        delay: i * streamDelay,
        blob,
        endTimeRef,
        duration,
        signal,
        maxNetworkRetries,
        retryDelayMs,
        warmUp,
        metricsState,
        startTime,
        onProgress,
        extra,
      }),
    );
  }
  await Promise.all(streams);

  const overheadFactor = detectOverheadFactor();
  const endNow = Math.min(performance.now(), endTimeRef.value);
  const actualMeasureStart =
    metricsState.measureStartTime > 0
      ? metricsState.measureStartTime
      : startTime;
  const measureTime = Math.max(
    TEST_CONFIG.MIN_MEASURE_SECONDS,
    (endNow - actualMeasureStart) / 1000,
  );
  const avgSpeed =
    (metricsState.totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;

  throwIfZeroBytes(metricsState, metricsState.totalBytes, {
    network: "Network error during upload. Try again or change server.",
    overload: "Server overloaded. Try again in a moment or change server.",
    noStreams: "Upload failed. No stream completed successfully.",
  });

  const stopReason = signal.aborted
    ? "aborted"
    : endTimeRef.value < nominalEndTime - 500
      ? "early_stable"
      : "duration";
  const diag = diagnostics.finish(stopReason);
  state.diagnostics = state.diagnostics || {};
  state.diagnostics.upload = diag;

  return Math.max(avgSpeed, 0);
}
