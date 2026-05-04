/** Adaptive stream ramping for browser HTTP speed tests. */

import { TEST_CONFIG } from "./state.js";

function parseIntegerParam(params, name, fallback, min, max) {
  const raw = params.get(name);
  if (raw == null || raw === "") return { value: fallback, overridden: false };
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value)) return { value: fallback, overridden: false };
  return {
    value: Math.min(max, Math.max(min, value)),
    overridden: true,
  };
}

function clampInteger(value, fallback, min, max) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.min(max, Math.max(min, parsed));
}

function normalizeAdaptiveConfig(config) {
  return {
    rampDuration: clampInteger(
      config?.rampDuration,
      TEST_CONFIG.ADAPTIVE_RAMP_SECONDS,
      1,
      TEST_CONFIG.ADAPTIVE_MAX_RAMP_SECONDS,
    ),
    measureDuration: clampInteger(
      config?.measureDuration,
      TEST_CONFIG.ADAPTIVE_MEASURE_SECONDS,
      1,
      TEST_CONFIG.ADAPTIVE_MAX_MEASURE_SECONDS,
    ),
    measureDurationOverridden: config?.measureDurationOverridden === true,
    maxStreams: clampInteger(
      config?.maxStreams,
      TEST_CONFIG.ADAPTIVE_MAX_STREAMS,
      TEST_CONFIG.ADAPTIVE_MIN_STREAMS,
      TEST_CONFIG.ADAPTIVE_MAX_STREAMS,
    ),
    gainThreshold: Number.isFinite(config?.gainThreshold)
      ? config.gainThreshold
      : TEST_CONFIG.ADAPTIVE_GAIN_THRESHOLD,
  };
}

export function resolveAdaptiveConfig() {
  const params = new URLSearchParams(globalThis.location?.search || "");
  const ramp = parseIntegerParam(
    params,
    "rampDuration",
    TEST_CONFIG.ADAPTIVE_RAMP_SECONDS,
    1,
    TEST_CONFIG.ADAPTIVE_MAX_RAMP_SECONDS,
  );
  const measure = parseIntegerParam(
    params,
    "measureDuration",
    TEST_CONFIG.ADAPTIVE_MEASURE_SECONDS,
    1,
    TEST_CONFIG.ADAPTIVE_MAX_MEASURE_SECONDS,
  );
  const maxStreams = parseIntegerParam(
    params,
    "maxStreams",
    TEST_CONFIG.ADAPTIVE_MAX_STREAMS,
    TEST_CONFIG.ADAPTIVE_MIN_STREAMS,
    TEST_CONFIG.ADAPTIVE_MAX_STREAMS,
  );

  return {
    rampDuration: ramp.value,
    measureDuration: measure.value,
    measureDurationOverridden: measure.overridden,
    maxStreams: maxStreams.value,
    gainThreshold: TEST_CONFIG.ADAPTIVE_GAIN_THRESHOLD,
  };
}

function resolveMeasureDuration(bestMbps, config) {
  if (config.measureDurationOverridden) return config.measureDuration;
  if (bestMbps >= 10_000) return TEST_CONFIG.ADAPTIVE_FAST_MEASURE_SECONDS;
  if (bestMbps >= 1_000) return TEST_CONFIG.ADAPTIVE_GBPS_MEASURE_SECONDS;
  return config.measureDuration;
}

function nextStreamCount(current, maxStreams) {
  return Math.min(maxStreams, current * 2);
}

function shouldStopRamping(previousMbps, currentMbps, threshold) {
  if (!Number.isFinite(previousMbps) || previousMbps <= 0) return false;
  if (!Number.isFinite(currentMbps) || currentMbps <= 0) return true;
  return (currentMbps - previousMbps) / previousMbps < threshold;
}

export function streamDelayForIndex(index) {
  return Math.min(
    index * TEST_CONFIG.ADAPTIVE_STREAM_DELAY_MS,
    TEST_CONFIG.ADAPTIVE_MAX_STREAM_SPREAD_MS,
  );
}

export async function runAdaptiveHTTPTest(options) {
  const { direction, runWindow, onPhase, onMeasureStart, signal } = options;
  const config = options.config
    ? normalizeAdaptiveConfig(options.config)
    : resolveAdaptiveConfig();
  const ramp = [];
  let best = { streams: TEST_CONFIG.ADAPTIVE_MIN_STREAMS, mbps: 0 };
  let previousMbps = 0;
  let streams = TEST_CONFIG.ADAPTIVE_MIN_STREAMS;
  let announcedSaturation = false;

  while (!signal.aborted) {
    if (!announcedSaturation) {
      onPhase?.("Saturating", streams);
      announcedSaturation = true;
    }
    let mbps = 0;
    try {
      mbps = await runWindow({
        duration: config.rampDuration,
        streams,
        collectDiagnostics: false,
      });
    } catch (err) {
      if (best.mbps > 0) break;
      throw err;
    }

    ramp.push({ streams, mbps: Math.round(mbps * 100) / 100 });
    if (mbps > best.mbps) best = { streams, mbps };
    if (shouldStopRamping(previousMbps, mbps, config.gainThreshold)) break;

    const next = nextStreamCount(streams, config.maxStreams);
    if (next === streams) break;
    previousMbps = mbps;
    streams = next;
  }

  if (signal.aborted) throw new DOMException("Aborted", "AbortError");
  const measureDuration = resolveMeasureDuration(best.mbps, config);
  onPhase?.("Measuring", best.streams);
  onMeasureStart?.(best.streams);
  const mbps = await runWindow({
    duration: measureDuration,
    streams: best.streams,
    collectDiagnostics: true,
    adaptive: {
      direction,
      selectedStreams: best.streams,
      measureDuration,
      ramp,
    },
  });
  return Math.max(mbps, 0);
}
