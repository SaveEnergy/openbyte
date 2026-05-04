/** Speed test orchestration: direction phases and run loop. */

import { state, TEST_CONFIG } from "./state.js";
import {
  updateProgress,
  updateSpeed,
  showState,
  resetProgress,
  updateTestType,
} from "./ui.js";
import { resolveAdaptiveConfig } from "./speedtest-adaptive.js";
export { measureLatency } from "./speedtest-latency.js";
import { startLoadedLatencyProbe } from "./speedtest-latency.js";

let workerRunCounter = 0;

function setTestPhase(phase, label, className) {
  state.phase = phase;
  if (phase === "latency") {
    showState("testing");
  } else {
    resetProgress();
  }
  updateTestType(label, className);
}

function adaptivePhaseLabel(direction, stage) {
  const icon = direction === "download" ? "↓" : "↑";
  return `${icon} ${stage}`;
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

function mergeWorkerDiagnostics(diagnostics) {
  if (!diagnostics || typeof diagnostics !== "object") return;
  state.diagnostics = state.diagnostics
    ? { ...state.diagnostics, ...diagnostics }
    : { ...diagnostics };
}

function runWorkerSpeedTest(direction, onProgress, signal, callbacks) {
  if (typeof Worker === "undefined") {
    throw new TypeError("This browser does not support Web Workers.");
  }

  const id = `${Date.now()}-${++workerRunCounter}`;
  const worker = new Worker(new URL("./speedtest-worker.js", import.meta.url), {
    type: "module",
  });
  const config = resolveAdaptiveConfig();

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
      worker.postMessage({ type: "cancel", id });
      settle(reject, makeAbortError());
    };

    const onMessage = (event) => {
      const message = event.data || {};
      if (message.id !== id) return;

      if (message.type === "progress") {
        onProgress(message.bytes || 0);
      } else if (message.type === "phase") {
        callbacks.onPhase?.(message.stage, message.streams);
      } else if (message.type === "measureStart") {
        callbacks.onMeasureStart?.(message.streams);
      } else if (message.type === "result") {
        mergeWorkerDiagnostics(message.diagnostics);
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

    worker.postMessage({ type: "run", id, direction, config });
  });
}

async function runTest(direction, signal) {
  if (signal.aborted) throw new DOMException("Aborted", "AbortError");

  const startTime = performance.now();
  let lastUpdate = startTime;
  let lastBytes = 0;
  let ewmaSpeed = 0;
  const ewmaAlpha = TEST_CONFIG.EWMA_ALPHA;

  const progressTick = setInterval(() => {
    if (signal.aborted) return;
    updateProgress(0);
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
      onPhase: (stage) =>
        updateTestType(
          adaptivePhaseLabel(direction, stage),
          direction === "download" ? "downloading" : "uploading",
        ),
      onMeasureStart: startMeasureLatencyProbe,
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
