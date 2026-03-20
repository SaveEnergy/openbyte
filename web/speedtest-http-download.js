/** HTTP download test: chunk retries, streams, warmup/diagnostics. */

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
  applyHttpMeasureTick,
} from "./speedtest-http-shared.js";

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
  const { attemptIndex, attemptsLength, signal, streamState } = options;
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
  applyHttpMeasureTick(
    ctx.readState,
    ctx.warmUp,
    value.length,
    now,
    ctx.startTime,
    ctx.onProgress,
    {
      diagnostics: ctx.diagnostics,
      earlyStop: ctx.earlyStop,
      endTimeRef: ctx.endTimeRef,
    },
  );
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

  const stopReason = resolveStopReason(signal, endTimeRef, nominalEndTime);
  const diag = diagnostics.finish(stopReason);
  state.diagnostics = state.diagnostics || {};
  state.diagnostics.download = diag;

  return Math.max(avgSpeed, 0);
}
