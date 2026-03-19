/** HTTP upload test: blob streaming, retries, warmup/diagnostics. */

import { getApiBase, state, TEST_CONFIG } from "./state.js";
import {
  sleep,
  retryAfterMs,
  isNetworkError,
  fetchWithTimeout,
} from "./utils.js";
import { createWarmUpDetector, createEarlyStopDetector } from "./warmup.js";
import { createDiagnosticsCollector } from "./diagnostics.js";
import {
  resolveStreamsInner,
  resolveChunkSize,
  detectOverheadFactor,
  throwIfZeroBytes,
  resolveStopReason,
} from "./speedtest-http-shared.js";

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

function handleUploadNonOkResponse(res, metricsState, retryAfterFn) {
  if (res.status === 503 || res.status === 429) {
    metricsState.sawOverload = true;
    return { action: "overload_break", retryAfter: retryAfterFn(res, 500) };
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
    retryAfterFn,
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
  const hr = handleUploadNonOkResponse(res, metricsState, retryAfterFn);
  if (hr.action === "overload_break") {
    await sleep(hr.retryAfter);
    return { ok: false, overload: true };
  }
  return { ok: false, retry: true };
}

async function continueUploadAfterResponse(
  response,
  context,
  consecutiveErrors,
  maxNetworkRetries,
  retryDelayMs,
) {
  const pr = await processUploadResponse(response, context);
  if (pr.ok) {
    return { breakLoop: false, nextErrors: 0 };
  }
  if (pr.overload) {
    return { breakLoop: true, nextErrors: consecutiveErrors };
  }
  const nextErrors = consecutiveErrors + 1;
  if (nextErrors > maxNetworkRetries) {
    return { breakLoop: true, nextErrors };
  }
  await sleep(retryDelayMs);
  return { breakLoop: false, nextErrors };
}

async function continueUploadAfterCatch(
  error,
  metricsState,
  consecutiveErrors,
  maxNetworkRetries,
  retryDelayMs,
) {
  const hc = handleUploadCatchError(
    error,
    metricsState,
    consecutiveErrors,
    maxNetworkRetries,
  );
  if (hc.action === "break") {
    return { breakLoop: true, nextErrors: consecutiveErrors };
  }
  if (hc.action === "retry") {
    await sleep(retryDelayMs);
    return { breakLoop: false, nextErrors: hc.next };
  }
  throw error;
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
      const responseState = await continueUploadAfterResponse(
        res,
        {
          blob,
          metricsState,
          retryAfterFn: retryAfterMs,
          warmUp,
          startTime,
          onProgress,
          extra,
        },
        consecutiveErrors,
        maxNetworkRetries,
        retryDelayMs,
      );
      consecutiveErrors = responseState.nextErrors;
      if (responseState.breakLoop) {
        break;
      }
    } catch (e) {
      const catchState = await continueUploadAfterCatch(
        e,
        metricsState,
        consecutiveErrors,
        maxNetworkRetries,
        retryDelayMs,
      );
      consecutiveErrors = catchState.nextErrors;
      if (catchState.breakLoop) {
        break;
      }
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

  const stopReason = resolveStopReason(signal, endTimeRef, nominalEndTime);
  const diag = diagnostics.finish(stopReason);
  state.diagnostics = state.diagnostics || {};
  state.diagnostics.upload = diag;

  return Math.max(avgSpeed, 0);
}
