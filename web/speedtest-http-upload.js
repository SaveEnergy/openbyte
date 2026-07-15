/** HTTP upload test: blob streaming, retries, warm-up, and measurement. */

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
} from "./utils.js";
import {
  resolveChunkSize,
  detectOverheadFactor,
  throwIfZeroBytes,
  applyHttpMeasureIntervalTick,
  createWarmUpDetector,
  createEarlyStopDetector,
} from "./speedtest-http-shared.js";

let uploadPayloadCache = null;

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

function createUploadPayloadBlob(blobSize) {
  const chunks = [];
  for (let i = 0; i < blobSize; i += TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES) {
    const piece = new Uint8Array(
      Math.min(TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES, blobSize - i),
    );
    crypto.getRandomValues(piece);
    chunks.push(piece);
  }
  return new Blob(chunks);
}

function getUploadPayloadBlob(blobSize) {
  if (!uploadPayloadCache || uploadPayloadCache.size !== blobSize) {
    uploadPayloadCache = {
      size: blobSize,
      blob: createUploadPayloadBlob(blobSize),
    };
  }
  return uploadPayloadCache.blob;
}

function roundPayloadSize(bytes) {
  const unit = TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES;
  return Math.ceil(bytes / unit) * unit;
}

function resolveUploadPayloadSize(chunkSize, duration, adaptive) {
  if (duration < TEST_CONFIG.ADAPTIVE_FAST_MEASURE_SECONDS) return chunkSize;
  const minPayload = Math.max(chunkSize, TEST_CONFIG.UPLOAD_MIN_PAYLOAD_BYTES);
  const bestMbps = Number(adaptive?.bestMbps);
  const streams = Math.max(1, Number(adaptive?.selectedStreams) || 1);
  if (!Number.isFinite(bestMbps) || bestMbps <= 0) return minPayload;

  const perStreamBytesPerMs = (bestMbps * 1_000_000) / 8 / streams / 1000;
  const targetBytes =
    perStreamBytesPerMs * TEST_CONFIG.UPLOAD_TARGET_REQUEST_MS;
  return roundPayloadSize(
    Math.min(
      TEST_CONFIG.UPLOAD_MAX_PAYLOAD_BYTES,
      Math.max(minPayload, targetBytes),
    ),
  );
}

async function readUploadResponseBytes(res, fallbackBytes) {
  const text = await res.text().catch(() => "");
  if (!text) return fallbackBytes;
  try {
    const payload = JSON.parse(text);
    const bytes = Number(payload?.bytes);
    if (Number.isFinite(bytes) && bytes >= 0) {
      return Math.min(bytes, fallbackBytes);
    }
  } catch (err) {
    console.debug("upload response parse failed", err);
  }
  return fallbackBytes;
}

async function runSingleUploadStream(index, options) {
  const {
    blob,
    endTimeRef,
    duration,
    signal,
    warmUp,
    metricsState,
    onProgress,
    measureContext,
  } = options;
  await sleep(streamDelayForIndex(index));

  let consecutiveErrors = 0;
  while (performance.now() < endTimeRef.value && !signal.aborted) {
    try {
      const requestStart = performance.now();
      const res = await sendUploadRequest(blob, duration, signal);
      if (res.ok) {
        const uploadedBytes = await readUploadResponseBytes(res, blob.size);
        metricsState.successfulStreams += 1;
        applyHttpMeasureIntervalTick(
          metricsState,
          warmUp,
          uploadedBytes,
          requestStart,
          performance.now(),
          onProgress,
          measureContext,
        );
        consecutiveErrors = 0;
        continue;
      }

      await res.text().catch(() => {});
      if (res.status === 503 || res.status === 429) {
        metricsState.sawOverload = true;
        await sleep(retryAfterMs(res, 500));
        break;
      }

      consecutiveErrors += 1;
      if (consecutiveErrors > TEST_CONFIG.MAX_NETWORK_RETRIES) break;
      await sleep(TEST_CONFIG.NETWORK_RETRY_DELAY_MS);
    } catch (error) {
      if (error.name === "AbortError") break;
      if (!isNetworkError(error)) throw error;
      metricsState.sawNetworkError = true;
      consecutiveErrors += 1;
      if (consecutiveErrors > TEST_CONFIG.MAX_NETWORK_RETRIES) throw error;
      await sleep(TEST_CONFIG.NETWORK_RETRY_DELAY_MS);
    }
  }
}

async function runUploadWindow(options) {
  const {
    duration,
    streams,
    onProgress,
    signal,
    isRamp = false,
    adaptive,
  } = options;
  const startTime = performance.now();
  const chunkSize = resolveChunkSize();
  const blobSize = resolveUploadPayloadSize(chunkSize, duration, adaptive);
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
  const endTimeRef = { value: startTime + duration * 1000 };
  const measureContext = { earlyStop, endTimeRef };
  const blob = getUploadPayloadBlob(blobSize);
  const streamContext = {
    blob,
    endTimeRef,
    duration,
    signal,
    warmUp,
    metricsState,
    onProgress,
    measureContext,
  };

  const streamPromises = [];
  for (let i = 0; i < streams; i++) {
    streamPromises.push(runSingleUploadStream(i, streamContext));
  }
  await Promise.all(streamPromises);

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
  if (isRamp && metricsState.sawOverload) {
    throw new Error("Server overloaded during adaptive upload ramp");
  }

  return Math.max(avgSpeed, 0);
}

export async function runUploadTest(onProgress, signal, options = {}) {
  return runAdaptiveHTTPTest({
    signal,
    config: options.config,
    onPhase: options.onPhase,
    onMeasureStart: options.onMeasureStart,
    runWindow: (windowOptions) =>
      runUploadWindow({
        ...windowOptions,
        onProgress,
        signal,
      }),
  });
}
