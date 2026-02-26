/** HTTP download/upload test execution. */

import { apiBase, state, TEST_CONFIG } from "./state.js";
import {
  sleep,
  retryAfterMs,
  isNetworkError,
  fetchWithTimeout,
} from "./utils.js";

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

export function createWarmUpDetector(durationMs) {
  const windowMs = TEST_CONFIG.WARMUP_WINDOW_MS;
  const stabilityThreshold = TEST_CONFIG.WARMUP_STABILITY_THRESHOLD;
  const requiredStableWindows = TEST_CONFIG.WARMUP_REQUIRED_WINDOWS;
  const maxGraceMs = Math.min(
    durationMs * TEST_CONFIG.WARMUP_MAX_GRACE_RATIO,
    TEST_CONFIG.WARMUP_MAX_GRACE_MS,
  );

  let windowBytes = 0;
  let windowStart = 0;
  let detectorStart = 0;
  let recentSpeeds = [];
  let settled = false;

  return {
    settled() {
      return settled;
    },
    record(bytes, now) {
      if (settled) return;
      if (detectorStart === 0) {
        detectorStart = now;
        windowStart = now;
      }
      windowBytes += bytes;
      const windowElapsed = now - windowStart;

      if (windowElapsed >= windowMs) {
        const speed = (windowBytes * 8) / (windowElapsed / 1000);
        recentSpeeds.push(speed);
        windowBytes = 0;
        windowStart = now;

        if (recentSpeeds.length >= requiredStableWindows) {
          const recent = recentSpeeds.slice(-requiredStableWindows);
          const avg = recent.reduce((a, b) => a + b, 0) / recent.length;
          if (avg === 0) {
            settled = true;
            return;
          }
          const maxDev = Math.max(
            ...recent.map((s) => Math.abs(s - avg) / avg),
          );
          if (maxDev < stabilityThreshold) settled = true;
        }

        if (now - detectorStart > maxGraceMs) settled = true;
      }
    },
  };
}

function buildDownloadChunkAttempts(chunkSize) {
  const preferredFallback = 256 * 1024;
  const attempts = [chunkSize];
  if (preferredFallback < chunkSize) attempts.push(preferredFallback);
  if (65536 < (attempts.at(-1) ?? 0)) attempts.push(65536);
  return attempts;
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
      if (e.name === "AbortError" || signal.aborted) return "aborted";
      if (e.status === 503 || e.status === 429) {
        streamState.sawOverload = true;
        await sleep(e.retryAfter || 500);
        return "overloaded";
      }
      if (isNetworkError(e)) {
        streamState.sawNetworkError = true;
        if (retry < maxNetworkRetries) {
          await sleep(retryDelayMs);
          continue;
        }
      }
      if (attemptIndex < attemptsLength - 1) {
        console.warn("Download stream failed, retrying smaller chunk", e);
      } else {
        console.warn("Download stream failed after retries", e);
      }
      return "failed";
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

export async function runDownloadTest(duration, onProgress, signal) {
  const startTime = performance.now();
  const numStreams = resolveStreamsInner();
  const chunkSize = resolveChunkSize();
  const streamDelay = TEST_CONFIG.STREAM_DELAY_MS;
  const maxNetworkRetries = TEST_CONFIG.MAX_NETWORK_RETRIES;
  const retryDelayMs = TEST_CONFIG.NETWORK_RETRY_DELAY_MS;
  const endTime = startTime + duration * 1000;
  let totalBytes = 0;
  let allBytes = 0;
  const streamState = {
    sawNetworkError: false,
    sawOverload: false,
    successfulStreams: 0,
  };

  const warmUp = createWarmUpDetector(duration * 1000);
  let measureStartTime = 0;

  const downloadStream = async (chunk) => {
    const res = await fetchWithTimeout(
      `${apiBase}/download?duration=${duration}&chunk=${chunk}`,
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
    try {
      while (true) {
        if (signal.aborted) break;

        const now = performance.now();
        if (now >= endTime) {
          await reader.cancel();
          break;
        }

        const { done, value } = await reader.read();
        if (done) break;

        allBytes += value.length;

        if (warmUp.settled()) {
          totalBytes += value.length;
        } else {
          warmUp.record(value.length, now);
          if (warmUp.settled()) {
            totalBytes = 0;
            measureStartTime = now;
          }
        }

        const elapsedSec = (now - startTime) / 1000;
        const displayBytes = warmUp.settled() ? totalBytes : allBytes;
        onProgress(displayBytes, elapsedSec);
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
  const endNow = Math.min(performance.now(), endTime);
  const actualMeasureStart =
    measureStartTime > 0 ? measureStartTime : startTime;
  const measureTime = Math.max(
    TEST_CONFIG.MIN_MEASURE_SECONDS,
    (endNow - actualMeasureStart) / 1000,
  );
  const avgSpeed = (totalBytes * 8 * overheadFactor) / measureTime / 1_000_000;

  if (totalBytes === 0 && streamState.sawNetworkError) {
    throw new Error(
      "Network error during download. Try again or change server.",
    );
  }
  if (totalBytes === 0 && streamState.sawOverload) {
    throw new Error(
      "Server overloaded. Try again in a moment or change server.",
    );
  }
  if (totalBytes === 0 && streamState.successfulStreams === 0) {
    throw new Error("Download failed. No stream completed successfully.");
  }

  return Math.max(avgSpeed, 0);
}

async function sendUploadRequest(blob, duration, signal) {
  return fetchWithTimeout(
    `${apiBase}/upload`,
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
) {
  metricsState.allBytes += blobSize;
  metricsState.successfulStreams += 1;

  if (warmUp.settled()) {
    metricsState.totalBytes += blobSize;
  } else {
    warmUp.record(blobSize, now);
    if (warmUp.settled()) {
      metricsState.totalBytes = 0;
      metricsState.measureStartTime = now;
    }
  }

  const elapsedSec = (now - startTime) / 1000;
  const displayBytes = warmUp.settled()
    ? metricsState.totalBytes
    : metricsState.allBytes;
  onProgress(displayBytes, elapsedSec);
}

async function runSingleUploadStream(options) {
  const {
    delay,
    blob,
    endTime,
    duration,
    signal,
    maxNetworkRetries,
    retryDelayMs,
    warmUp,
    metricsState,
    startTime,
    onProgress,
  } = options;
  await new Promise((r) => setTimeout(r, delay));

  let consecutiveErrors = 0;
  while (performance.now() < endTime && !signal.aborted) {
    try {
      const res = await sendUploadRequest(blob, duration, signal);
      if (!res.ok) {
        await res.text().catch(() => {});
        if (res.status === 503 || res.status === 429) {
          metricsState.sawOverload = true;
          await sleep(retryAfterMs(res, 500));
          break;
        }
        consecutiveErrors += 1;
        if (consecutiveErrors > maxNetworkRetries) break;
        await sleep(retryDelayMs);
        continue;
      }

      consecutiveErrors = 0;
      await res.text().catch(() => {});
      const now = performance.now();
      recordUploadProgress(
        metricsState,
        warmUp,
        blob.size,
        now,
        startTime,
        onProgress,
      );
    } catch (e) {
      if (e.name === "AbortError") break;
      if (isNetworkError(e)) {
        metricsState.sawNetworkError = true;
        consecutiveErrors += 1;
        if (consecutiveErrors <= maxNetworkRetries) {
          await sleep(retryDelayMs);
          continue;
        }
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

  const chunks = [];
  for (let i = 0; i < blobSize; i += TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES) {
    const piece = new Uint8Array(
      Math.min(TEST_CONFIG.UPLOAD_RANDOM_CHUNK_BYTES, blobSize - i),
    );
    crypto.getRandomValues(piece);
    chunks.push(piece);
  }
  const blob = new Blob(chunks);

  const endTime = startTime + duration * 1000;

  const streams = [];
  for (let i = 0; i < numStreams; i++) {
    streams.push(
      runSingleUploadStream({
        delay: i * streamDelay,
        blob,
        endTime,
        duration,
        signal,
        maxNetworkRetries,
        retryDelayMs,
        warmUp,
        metricsState,
        startTime,
        onProgress,
      }),
    );
  }
  await Promise.all(streams);

  const overheadFactor = detectOverheadFactor();
  const endNow = Math.min(performance.now(), endTime);
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

  if (metricsState.totalBytes === 0 && metricsState.sawNetworkError) {
    throw new Error("Network error during upload. Try again or change server.");
  }
  if (metricsState.totalBytes === 0 && metricsState.sawOverload) {
    throw new Error(
      "Server overloaded. Try again in a moment or change server.",
    );
  }
  if (metricsState.totalBytes === 0 && metricsState.successfulStreams === 0) {
    throw new Error("Upload failed. No stream completed successfully.");
  }

  return Math.max(avgSpeed, 0);
}
