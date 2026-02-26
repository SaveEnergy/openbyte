/** Speed test orchestration: direction phases and run loop. */

import { state, TEST_CONFIG } from "./state.js";
import {
  updateProgress,
  updateSpeed,
  showState,
  resetProgress,
  updateTestType,
} from "./ui.js";
import { runDownloadTest, runUploadTest } from "./speedtest-http.js";
import {
  startLoadedLatencyProbe,
  measureLatency,
} from "./speedtest-latency.js";

export { measureLatency };

function setTestPhase(phase, label, className) {
  state.phase = phase;
  if (phase === "latency") {
    showState("testing");
  } else {
    resetProgress();
  }
  updateTestType(label, className);
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

async function runTest(direction, signal) {
  if (signal.aborted) throw new Error("Test cancelled");

  const duration = state.settings.duration;
  const startTime = performance.now();
  let lastUpdate = startTime;
  let lastBytes = 0;
  let ewmaSpeed = 0;
  const ewmaAlpha = TEST_CONFIG.EWMA_ALPHA;

  const progressTick = setInterval(() => {
    if (signal.aborted) return;
    const elapsed = (performance.now() - startTime) / 1000;
    updateProgress(Math.min(100, (elapsed / duration) * 100));
  }, TEST_CONFIG.PROGRESS_TICK_MS);

  const onProgress = (bytes, elapsed) => {
    if (elapsed > duration) return;
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

      updateProgress(Math.min(100, (elapsed / duration) * 100));

      lastUpdate = now;
      lastBytes = Math.max(lastBytes, bytes);
    }
  };

  const latencyProbe = startLoadedLatencyProbe(signal);

  let result;
  try {
    if (direction === "download") {
      result = await runDownloadTest(duration, onProgress, signal);
    } else {
      result = await runUploadTest(duration, onProgress, signal);
    }
  } finally {
    clearInterval(progressTick);
    await latencyProbe.stop();
  }
  const loadedLatency = latencyProbe.getMedian();
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
