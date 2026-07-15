/** HTTP download test: chunk retries, streams, warm-up, and measurement. */

import { getApiBase, TEST_CONFIG } from "./state.js";
import {
  runAdaptiveHTTPTest,
  streamDelayForIndex,
} from "./speedtest-adaptive.js";
import {
  sleep,
  retryAfterMs,
  isNetworkError,
  fetchWithTimeout,
  createCodedError,
} from "./utils.js";
import {
  resolveChunkSize,
  throwIfZeroBytes,
  applyHttpMeasureTick,
  createWarmUpDetector,
  createEarlyStopDetector,
} from "./speedtest-http-shared.js";

function buildDownloadChunkAttempts(chunkSize) {
  const preferredFallback = 256 * 1024;
  const attempts = [chunkSize];
  if (preferredFallback < chunkSize) attempts.push(preferredFallback);
  if (65536 < (attempts.at(-1) ?? 0)) attempts.push(65536);
  return attempts;
}

async function runDownloadStream(downloadChunk, signal, streamState) {
  const attempts = buildDownloadChunkAttempts(resolveChunkSize());
  for (let attemptIndex = 0; attemptIndex < attempts.length; attemptIndex++) {
    for (let retry = 0; retry <= TEST_CONFIG.MAX_NETWORK_RETRIES; retry++) {
      if (signal.aborted) return "aborted";
      try {
        if (await downloadChunk(attempts[attemptIndex])) return "success";
        break;
      } catch (error) {
        if (error.name === "AbortError" || signal.aborted) return "aborted";
        if (error.status === 503 || error.status === 429) {
          streamState.sawOverload = true;
          await sleep(error.retryAfter || 500);
          return "overloaded";
        }
        if (isNetworkError(error)) {
          streamState.sawNetworkError = true;
          if (retry < TEST_CONFIG.MAX_NETWORK_RETRIES) {
            await sleep(TEST_CONFIG.NETWORK_RETRY_DELAY_MS);
            continue;
          }
          break;
        }
        console.warn(
          attemptIndex < attempts.length - 1
            ? "Download stream failed, retrying smaller chunk"
            : "Download stream failed after retries",
          error,
        );
        break;
      }
    }
  }
  return "failed";
}

function acquireDownloadReader(body) {
  // BYOB readers reuse one large buffer, coalescing reads and avoiding a
  // fresh allocation per chunk on the multi-Gbit/s hot path.
  try {
    return { reader: body.getReader({ mode: "byob" }), byob: true };
  } catch {
    return { reader: body.getReader(), byob: false };
  }
}

async function readNextDownloadChunk(reader, byob, bufferRef) {
  if (!byob) return reader.read();
  const result = await reader.read(new Uint8Array(bufferRef.value));
  // The buffer is transferred (detached) on each read; keep the returned one.
  if (result.value) bufferRef.value = result.value.buffer;
  return result;
}

async function runDownloadWindow(options) {
  const {
    duration,
    streams,
    onProgress,
    signal,
    isRamp = false,
  } = options;
  const startTime = performance.now();
  const endTimeRef = { value: startTime + duration * 1000 };
  const streamState = {
    sawNetworkError: false,
    sawOverload: false,
    successfulStreams: 0,
  };

  const warmUp = createWarmUpDetector(duration * 1000);
  const earlyStop = createEarlyStopDetector(() => warmUp.settled());
  const readState = { totalBytes: 0, measureStartTime: 0, allBytes: 0 };
  const measureContext = { endTimeRef, earlyStop };

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

    const { reader, byob } = acquireDownloadReader(res.body);
    const bufferRef = {
      value: byob
        ? new ArrayBuffer(TEST_CONFIG.DOWNLOAD_READ_BUFFER_BYTES)
        : null,
    };
    try {
      while (true) {
        if (signal.aborted) break;
        const now = performance.now();
        if (now >= endTimeRef.value) {
          await reader.cancel();
          break;
        }
        const { done, value } = await readNextDownloadChunk(
          reader,
          byob,
          bufferRef,
        );
        if (done) break;
        applyHttpMeasureTick(
          readState,
          warmUp,
          value.length,
          now,
          onProgress,
          measureContext,
        );
      }
    } finally {
      reader.releaseLock();
    }
    return true;
  };

  const streamPromises = [];
  for (let i = 0; i < streams; i++) {
    streamPromises.push(
      (async () => {
        await sleep(streamDelayForIndex(i));
        const result = await runDownloadStream(
          downloadStream,
          signal,
          streamState,
        );
        if (result === "success") streamState.successfulStreams += 1;
      })(),
    );
  }

  await Promise.all(streamPromises);

  const endNow = Math.min(performance.now(), endTimeRef.value);
  const { totalBytes } = readState;
  const actualMeasureStart =
    readState.measureStartTime > 0 ? readState.measureStartTime : startTime;
  const measureTime = Math.max(
    TEST_CONFIG.MIN_MEASURE_SECONDS,
    (endNow - actualMeasureStart) / 1000,
  );
  const avgSpeed = (totalBytes * 8) / measureTime / 1_000_000;

  throwIfZeroBytes(streamState, totalBytes, "download");
  if (isRamp && streamState.sawOverload) {
    throw createCodedError("server.overloaded");
  }

  return Math.max(avgSpeed, 0);
}

export async function runDownloadTest(onProgress, signal, options = {}) {
  return runAdaptiveHTTPTest({
    signal,
    config: options.config,
    onPhase: options.onPhase,
    onMeasureStart: options.onMeasureStart,
    runWindow: (windowOptions) =>
      runDownloadWindow({
        ...windowOptions,
        onProgress,
        signal,
      }),
  });
}
