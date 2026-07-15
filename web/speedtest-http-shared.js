/** Shared helpers for HTTP speed tests (download + upload). */

import { TEST_CONFIG } from "./state.js";
import { createCodedError } from "./utils.js";

const TRANSFER_ERROR_MESSAGES = {
  download: {
    network: "Network error during download. Please try again.",
    overload: "Server overloaded during download. Please try again shortly.",
    noStreams: "Download failed. No stream completed successfully.",
  },
  upload: {
    network: "Network error during upload. Please try again.",
    overload: "Server overloaded during upload. Please try again shortly.",
    noStreams: "Upload failed. No stream completed successfully.",
  },
};

export function resolveChunkSize() {
  return 1024 * 1024;
}

export function detectOverheadFactor() {
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

export function throwIfZeroBytes(streamState, totalBytes, direction) {
  if (totalBytes > 0) return;
  const messages = TRANSFER_ERROR_MESSAGES[direction];
  if (!messages) return;
  if (streamState.sawNetworkError) {
    throw createCodedError(`${direction}.network`, messages.network);
  }
  if (streamState.sawOverload) {
    throw createCodedError("server.overloaded", messages.overload);
  }
  if (streamState.successfulStreams === 0) {
    throw createCodedError(`${direction}.noStreams`, messages.noStreams);
  }
}

/**
 * Shared warmup + measure accounting + progress callback for HTTP upload/download.
 * `metricsState` must have `allBytes`, `totalBytes`, `measureStartTime`.
 */
export function applyHttpMeasureTick(
  metricsState,
  warmUp,
  byteCount,
  now,
  onProgress,
  extra,
) {
  const measuring = warmUp.settled();
  metricsState.allBytes += byteCount;
  if (measuring) {
    metricsState.totalBytes += byteCount;
  } else {
    warmUp.record(byteCount, now);
    if (warmUp.settled()) {
      metricsState.totalBytes = 0;
      metricsState.measureStartTime = now;
    }
  }
  if (extra?.earlyStop && measuring && extra.earlyStop.record(byteCount, now)) {
    extra.endTimeRef.value = Math.min(now, extra.endTimeRef.value);
  }
  const displayBytes = measuring
    ? metricsState.totalBytes
    : metricsState.allBytes;
  onProgress(displayBytes);
}

export function measuredIntervalBytes(
  byteCount,
  intervalStart,
  intervalEnd,
  measureStart,
  measureEnd,
) {
  if (byteCount <= 0) return 0;
  const intervalMs = intervalEnd - intervalStart;
  if (intervalMs <= 0) return 0;

  const overlapStart = Math.max(intervalStart, measureStart);
  const overlapEnd = Math.min(intervalEnd, measureEnd);
  if (overlapEnd <= overlapStart) return 0;

  return byteCount * ((overlapEnd - overlapStart) / intervalMs);
}

/**
 * Upload fetch only reports completion, not socket-level upload progress.
 * Attribute each completed request by the part of its lifetime that overlaps
 * the measured window so warm-up and post-deadline bytes do not inflate Mbps.
 */
export function applyHttpMeasureIntervalTick(
  metricsState,
  warmUp,
  byteCount,
  intervalStart,
  intervalEnd,
  onProgress,
  extra,
) {
  const wasMeasuring = warmUp.settled();
  metricsState.allBytes += byteCount;

  if (wasMeasuring) {
    const measureStart =
      metricsState.measureStartTime > 0
        ? metricsState.measureStartTime
        : intervalStart;
    const measureEnd = extra?.endTimeRef?.value ?? intervalEnd;
    const measuredBytes = measuredIntervalBytes(
      byteCount,
      intervalStart,
      intervalEnd,
      measureStart,
      measureEnd,
    );
    metricsState.totalBytes += measuredBytes;

    if (
      extra?.earlyStop &&
      measuredBytes > 0 &&
      extra.earlyStop.record(measuredBytes, intervalEnd)
    ) {
      extra.endTimeRef.value = Math.min(intervalEnd, extra.endTimeRef.value);
    }
  } else {
    warmUp.record(byteCount, intervalEnd);
    if (warmUp.settled()) {
      metricsState.totalBytes = 0;
      metricsState.measureStartTime = intervalEnd;
    }
  }

  const displayBytes = wasMeasuring
    ? metricsState.totalBytes
    : metricsState.allBytes;
  onProgress(displayBytes);
}

const EARLY_STOP_WINDOW_MS = 500;
const EARLY_STOP_DELTA_THRESHOLD = 0.05;
const EARLY_STOP_STABLE_WINDOWS = 3;
const EARLY_STOP_MIN_WINDOWS = 2;

export function createEarlyStopDetector(settledFn) {
  let windowBytes = 0;
  let windowStart = 0;
  const recentSpeeds = [];

  return {
    record(bytes, now) {
      if (!settledFn()) return false;
      if (windowStart === 0) windowStart = now;
      windowBytes += bytes;
      const elapsed = now - windowStart;
      if (elapsed < EARLY_STOP_WINDOW_MS) return false;
      const speed = (windowBytes * 8) / (elapsed / 1000);
      recentSpeeds.push(speed);
      windowBytes = 0;
      windowStart = now;
      const recent = recentSpeeds.slice(-EARLY_STOP_STABLE_WINDOWS);
      if (
        recentSpeeds.length < EARLY_STOP_MIN_WINDOWS ||
        recent.length < EARLY_STOP_STABLE_WINDOWS
      ) {
        return false;
      }
      const avg = recent.reduce((a, b) => a + b, 0) / recent.length;
      if (avg <= 0) return false;
      const maxDelta = Math.max(...recent.map((s) => Math.abs(s - avg) / avg));
      return maxDelta < EARLY_STOP_DELTA_THRESHOLD;
    },
  };
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
  const recentSpeeds = [];
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
