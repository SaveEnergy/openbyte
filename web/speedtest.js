/** Speed test orchestration: direction phases and run loop. */

import { getApiBase, state, TEST_CONFIG } from "./state.js";
import {
  updateProgress,
  updateSpeed,
  showState,
  resetProgress,
  updateTestType,
} from "./ui.js";
import { resolveAdaptiveConfig } from "./speedtest-adaptive.js";
import { fetchWithTimeout } from "./utils.js";
import { getNextHopProtocol, updateNetworkDisplay } from "./network.js";

/** Portion of a direction phase's progress allotted to the ramp-up stage. */
const RAMP_PROGRESS_PORTION = 0.45;

function setTestPhase(phase, label, className) {
  state.phase = phase;
  if (phase === "latency") {
    showState("testing");
  } else {
    resetProgress();
  }
  updateTestType(label, className);
}

function adaptivePhaseLabel(direction, stage, streams) {
  const icon = direction === "download" ? "↓" : "↑";
  const streamsSuffix =
    Number.isFinite(streams) && streams > 1 ? ` ×${streams}` : "";
  return `${icon} ${stage}${streamsSuffix}`;
}

function createDirectionProgressModel() {
  const now = performance.now();
  return {
    windowIndex: 0,
    maxWindows: 1,
    windowStartedAt: now,
    rampDurationMs: TEST_CONFIG.ADAPTIVE_RAMP_SECONDS * 1000,
    measureStartedAt: 0,
    measureDurationMs: 0,
    highWaterMark: 0,
  };
}

function noteRampWindow(model, info) {
  if (!info) return;
  if (Number.isFinite(info.windowIndex)) model.windowIndex = info.windowIndex;
  if (Number.isFinite(info.maxWindows) && info.maxWindows >= 1) {
    model.maxWindows = info.maxWindows;
  }
  if (Number.isFinite(info.rampDuration) && info.rampDuration > 0) {
    model.rampDurationMs = info.rampDuration * 1000;
  }
  model.windowStartedAt = performance.now();
}

function noteMeasureStart(model, duration) {
  model.measureStartedAt = performance.now();
  model.measureDurationMs =
    Number.isFinite(duration) && duration > 0 ? duration * 1000 : 0;
}

/**
 * Progress estimate for a direction phase: the ramp stage advances by
 * completed windows (bounded by maxWindows), the measure stage by elapsed
 * time over its known duration. Monotonic so early ramp exit jumps forward.
 */
function directionProgressPercent(model) {
  const now = performance.now();
  let progress;
  if (model.measureStartedAt > 0 && model.measureDurationMs > 0) {
    const measured = Math.min(
      (now - model.measureStartedAt) / model.measureDurationMs,
      1,
    );
    progress = RAMP_PROGRESS_PORTION + (1 - RAMP_PROGRESS_PORTION) * measured;
  } else {
    const windowElapsed = Math.min(
      (now - model.windowStartedAt) / model.rampDurationMs,
      1,
    );
    progress =
      RAMP_PROGRESS_PORTION *
      Math.min((model.windowIndex + windowElapsed) / model.maxWindows, 1);
  }
  model.highWaterMark = Math.max(model.highWaterMark, Math.min(progress, 1));
  return model.highWaterMark * 100;
}

function nextFrame() {
  return new Promise((resolve) => requestAnimationFrame(resolve));
}

export async function runDirectionPhase(
  signal,
  phase,
  label,
  className,
  direction,
) {
  setTestPhase(phase, label, className);
  await nextFrame();
  return runTest(direction, signal);
}

function makeAbortError() {
  return new DOMException("Aborted", "AbortError");
}

function workerMessageError(message) {
  if (message.name === "AbortError") return makeAbortError();
  const error = new Error(message.message || "Speed test worker failed");
  error.name = message.name || "Error";
  return error;
}

function runWorkerSpeedTest(direction, onProgress, signal, callbacks) {
  if (typeof Worker === "undefined") {
    throw new TypeError("This browser does not support Web Workers.");
  }

  const worker = new Worker(new URL("./speedtest-worker.js", import.meta.url), {
    type: "module",
  });
  const config = {
    ...resolveAdaptiveConfig(),
    nextHopProtocol: getNextHopProtocol(),
  };

  return new Promise((resolve, reject) => {
    let settled = false;

    const cleanup = () => {
      signal.removeEventListener("abort", onAbort);
      worker.removeEventListener("message", onMessage);
      worker.removeEventListener("error", onError);
      worker.removeEventListener("messageerror", onMessageError);
      worker.terminate();
    };

    const settle = (fn, value) => {
      if (settled) return;
      settled = true;
      cleanup();
      fn(value);
    };

    const onAbort = () => {
      settle(reject, makeAbortError());
    };

    const onMessage = (event) => {
      const message = event.data || {};

      if (message.type === "progress") {
        onProgress(message.bytes || 0);
      } else if (message.type === "phase") {
        callbacks.onPhase?.(message.stage, message.streams, message);
      } else if (message.type === "measureStart") {
        callbacks.onMeasureStart?.(message.streams, message.duration);
      } else if (message.type === "result") {
        settle(resolve, Math.max(message.mbps || 0, 0));
      } else if (message.type === "error") {
        settle(reject, workerMessageError(message));
      }
    };

    const onError = (event) => {
      settle(reject, new Error(event.message || "Speed test worker failed"));
    };

    const onMessageError = () => {
      settle(reject, new Error("Speed test worker sent an unreadable message"));
    };

    worker.addEventListener("message", onMessage);
    worker.addEventListener("error", onError);
    worker.addEventListener("messageerror", onMessageError);
    signal.addEventListener("abort", onAbort, { once: true });

    if (signal.aborted) {
      onAbort();
      return;
    }

    worker.postMessage({ direction, config });
  });
}

async function runTest(direction, signal) {
  if (signal.aborted) throw new DOMException("Aborted", "AbortError");

  const startTime = performance.now();
  let lastUpdate = startTime;
  let lastBytes = 0;
  let ewmaSpeed = 0;
  const ewmaAlpha = TEST_CONFIG.EWMA_ALPHA;

  const progressModel = createDirectionProgressModel();
  const progressTick = setInterval(() => {
    if (signal.aborted) return;
    updateProgress(directionProgressPercent(progressModel));
  }, TEST_CONFIG.PROGRESS_TICK_MS);

  const onProgress = (bytes) => {
    const now = performance.now();
    const intervalMs = now - lastUpdate;

    if (intervalMs >= TEST_CONFIG.SPEED_UPDATE_MIN_INTERVAL_MS) {
      if (bytes < lastBytes) {
        lastBytes = bytes;
        ewmaSpeed = 0;
      }
      const intervalBytes = bytes - lastBytes;

      if (intervalBytes > 0 && intervalMs > 0) {
        const instantSpeed =
          (intervalBytes * 8) / (intervalMs / 1000) / 1_000_000;
        ewmaSpeed =
          ewmaSpeed === 0
            ? instantSpeed
            : ewmaAlpha * instantSpeed + (1 - ewmaAlpha) * ewmaSpeed;
        updateSpeed(Math.max(0, ewmaSpeed), direction);
      }

      lastUpdate = now;
      lastBytes = Math.max(lastBytes, bytes);
    }
  };

  let latencyProbe = null;
  const startMeasureLatencyProbe = () => {
    if (latencyProbe || signal.aborted) return;
    latencyProbe = startLoadedLatencyProbe(signal);
  };
  let result;
  try {
    result = await runWorkerSpeedTest(direction, onProgress, signal, {
      onPhase: (stage, streams, info) => {
        noteRampWindow(progressModel, info);
        updateTestType(
          adaptivePhaseLabel(direction, stage, streams),
          direction === "download" ? "downloading" : "uploading",
        );
      },
      onMeasureStart: (streams, duration) => {
        noteMeasureStart(progressModel, duration);
        startMeasureLatencyProbe();
      },
    });
  } finally {
    clearInterval(progressTick);
    if (latencyProbe) await latencyProbe.stop();
  }
  const loadedLatency = latencyProbe ? latencyProbe.getMedian() : 0;
  if (direction === "download") {
    state.downloadLatency = loadedLatency;
  } else {
    state.uploadLatency = loadedLatency;
  }

  if (state.isRunning) {
    updateProgress(100);
  }

  return result;
}

function filterOutliersIQR(samples) {
  if (samples.length < 4) return samples.slice();
  const sorted = samples.slice().sort((a, b) => a - b);
  const q1 = sorted[Math.floor(sorted.length * 0.25)];
  const q3 = sorted[Math.floor(sorted.length * 0.75)];
  const iqr = q3 - q1;
  const lower = q1 - 1.5 * iqr;
  const upper = q3 + 1.5 * iqr;
  return samples.filter((s) => s >= lower && s <= upper);
}

async function captureClientIPIfNeeded(res, capturedRef, signal) {
  if (capturedRef.captured || !res.ok) {
    await res.text().catch(() => {});
    return;
  }
  try {
    const data = await res.json();
    if (data.client_ip && state.abortController?.signal === signal) {
      state.networkInfo[data.ipv6 ? "ipv6" : "ipv4"] = data.client_ip;
      updateNetworkDisplay();
      capturedRef.captured = true;
    }
  } catch (err) {
    console.debug("failed to parse ping response", err);
    await res.text().catch(() => {});
  }
}

function computeJitter(samples) {
  if (samples.length < 2) return 0;
  let sumDiff = 0;
  for (let i = 1; i < samples.length; i++) {
    sumDiff += Math.abs(samples[i] - samples[i - 1]);
  }
  return sumDiff / (samples.length - 1);
}

function sleepWithSignal(ms, signal) {
  if (ms <= 0 || !signal || signal.aborted) return Promise.resolve();
  return new Promise((resolve) => {
    let settled = false;
    const complete = () => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      signal.removeEventListener("abort", complete);
      resolve();
    };
    const timer = setTimeout(complete, ms);
    signal.addEventListener("abort", complete, { once: true });
  });
}

function startLoadedLatencyProbe(signal) {
  const samples = [];
  let running = true;
  const probeController = new AbortController();
  const onAbort = () => probeController.abort();
  if (signal.aborted) {
    probeController.abort();
  } else {
    signal.addEventListener("abort", onAbort, { once: true });
  }

  const loop = async () => {
    try {
      while (running && !signal.aborted) {
        const start = performance.now();
        try {
          const res = await fetchWithTimeout(
            `${getApiBase()}/ping`,
            {
              method: "GET",
              cache: "no-store",
              signal: probeController.signal,
            },
            TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
          );
          samples.push(performance.now() - start);
          await res.text().catch(() => {});
        } catch (err) {
          console.debug("loaded latency probe ping failed", err);
          if (!running || signal.aborted || probeController.signal.aborted) {
            break;
          }
        }
        await sleepWithSignal(
          TEST_CONFIG.LOADED_LATENCY_POLL_MS,
          probeController.signal,
        );
      }
    } finally {
      signal.removeEventListener("abort", onAbort);
    }
  };

  const promise = loop();

  return {
    async stop() {
      running = false;
      probeController.abort();
      await promise;
    },
    getMedian() {
      if (samples.length === 0) return 0;
      const filtered = filterOutliersIQR(samples);
      if (filtered.length === 0) return 0;
      filtered.sort((a, b) => a - b);
      return filtered[Math.floor(filtered.length / 2)];
    },
  };
}

export async function measureLatency(signal) {
  const rawSamples = [];
  const numSamples = TEST_CONFIG.LATENCY_SAMPLE_COUNT;
  const warmUpPings = TEST_CONFIG.LATENCY_WARMUP_PINGS;
  const capturedRef = { captured: false };

  for (let i = 0; i < numSamples; i++) {
    if (signal.aborted) break;

    const start = performance.now();
    try {
      const res = await fetch(`${getApiBase()}/ping`, {
        method: "GET",
        signal,
      });
      const rtt = performance.now() - start;

      await captureClientIPIfNeeded(res, capturedRef, signal);

      rawSamples.push(rtt);
      updateProgress((i / numSamples) * 100);
      updateSpeed(rtt, "latency");
    } catch (e) {
      if (e.name === "AbortError") break;
    }
  }

  if (rawSamples.length === 0) return { value: null, jitter: null };

  const samples =
    rawSamples.length > warmUpPings
      ? rawSamples.slice(warmUpPings)
      : rawSamples;

  const filtered = filterOutliersIQR(samples);
  const jitter = computeJitter(filtered);

  filtered.sort((a, b) => a - b);
  return { value: filtered[Math.floor(filtered.length / 2)], jitter };
}
