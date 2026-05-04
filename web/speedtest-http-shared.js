/** Shared helpers for HTTP speed tests (download + upload). */

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

export function attachAdaptiveDiagnostics(diag, adaptive, streams) {
  const out = { ...diag, streams };
  if (adaptive) {
    out.adaptive = adaptive;
  }
  return out;
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
  if (extra?.diagnostics) {
    extra.diagnostics.record(byteCount, now, measuring);
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

    if (extra?.diagnostics) {
      extra.diagnostics.record(measuredBytes, intervalEnd, measuredBytes > 0);
    }
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
    if (extra?.diagnostics) {
      extra.diagnostics.record(byteCount, intervalEnd, false);
    }
  }

  const displayBytes = wasMeasuring
    ? metricsState.totalBytes
    : metricsState.allBytes;
  onProgress(displayBytes);
}
