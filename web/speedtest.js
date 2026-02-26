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
export { measureLatency } from "./speedtest-latency.js";
import { startLoadedLatencyProbe } from "./speedtest-latency.js";

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
  let hasTransferProgress = false;
  let transferProgress = 0;
  let maxProgress = 0;
  const ewmaAlpha = TEST_CONFIG.EWMA_ALPHA;
  const clampProgress = (value) => Math.min(100, Math.max(0, value));
  const commitProgress = (candidate) => {
    if (!Number.isFinite(candidate)) return;
    maxProgress = Math.max(maxProgress, clampProgress(candidate));
    updateProgress(maxProgress);
  };

  const progressTick = setInterval(() => {
    if (signal.aborted) return;
    if (hasTransferProgress) {
      commitProgress(transferProgress);
      return;
    }
    const elapsedSec = (performance.now() - startTime) / 1000;
    commitProgress((elapsedSec / duration) * 100);
  }, TEST_CONFIG.PROGRESS_TICK_MS);

  const onProgress = (bytes, elapsed, phaseProgress) => {
    if (elapsed > duration) return;
    if (Number.isFinite(phaseProgress)) {
      hasTransferProgress = true;
      transferProgress = Math.max(
        transferProgress,
        clampProgress(phaseProgress),
      );
    }
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

      if (hasTransferProgress) {
        commitProgress(transferProgress);
      } else {
        commitProgress((elapsed / duration) * 100);
      }

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
    commitProgress(100);
  }

  return result;
}
