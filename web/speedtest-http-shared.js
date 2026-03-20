/** Shared helpers for HTTP speed tests (download + upload). */

import { state } from "./state.js";

export function resolveStreamsInner() {
  if (!state.settings.streams || Number.isNaN(state.settings.streams)) {
    return 4;
  }
  return state.settings.streams;
}

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

export function throwIfZeroBytes(streamState, totalBytes, messages) {
  if (totalBytes > 0) return;
  if (streamState.sawNetworkError) throw new Error(messages.network);
  if (streamState.sawOverload) throw new Error(messages.overload);
  if (streamState.successfulStreams === 0) throw new Error(messages.noStreams);
}

export function resolveStopReason(signal, endTimeRef, nominalEndTime) {
  if (signal.aborted) return "aborted";
  if (endTimeRef.value < nominalEndTime - 500) return "early_stable";
  return "duration";
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
  startTime,
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
  if (extra?.diagnostics) {
    extra.diagnostics.record(byteCount, now, measuring);
  }
  if (extra?.earlyStop && measuring && extra.earlyStop.record(byteCount, now)) {
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
