/** DOM updates: progress, speed display, state views, toast. */

import {
  state,
  elements,
  RING_CIRCUMFERENCE,
  RING_END_OFFSET,
  TEST_CONFIG,
  toast,
} from "./state.js";
import { computeBufferbloatGrade, formatSpeed } from "./utils.js";
import { updateNetworkDisplay } from "./network.js";

let visualProgress = 0;
let progressAnimationFrame = 0;

function focusStateAction(stateName) {
  if (elements.settingsModal?.open) return;
  const targets = {
    idle: elements.startBtn,
    testing: elements.cancelBtn,
    results: elements.restartBtn,
  };
  const target = targets[stateName];
  if (!target || typeof target.focus !== "function") return;
  requestAnimationFrame(() => {
    if (!target.classList.contains("hidden")) {
      target.focus();
    }
  });
}

function clearToastTimer() {
  if (toast.timer) {
    clearTimeout(toast.timer);
    toast.timer = null;
  }
}

function getToastElements(isError) {
  if (isError) {
    return {
      toastEl: elements.errorToast,
      messageEl: elements.errorMessage,
      duration: TEST_CONFIG.TOAST_ERROR_MS,
    };
  }
  return {
    toastEl: elements.successToast,
    messageEl: elements.successMessage,
    duration: TEST_CONFIG.TOAST_SUCCESS_MS,
  };
}

function formatLatencyMs(val) {
  if (typeof val === "number" && Number.isFinite(val) && val > 0) {
    return `${val.toFixed(1)} ms`;
  }
  return "-";
}

export function updateSpeed(speed, direction) {
  if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
    speed = 0;
  if (!elements.speedNumber || !elements.speedUnit) return;
  state.currentSpeed = speed;

  let displaySpeed, unit;

  if (direction === "latency") {
    displaySpeed = speed.toFixed(0);
    unit = "ms";
    elements.speedNumber.className = "speed-number measuring";
  } else if (speed >= 1000) {
    displaySpeed = (speed / 1000).toFixed(2);
    unit = "Gbps";
    elements.speedNumber.className =
      "speed-number " +
      (direction === "download" ? "downloading" : "uploading");
  } else {
    displaySpeed = speed.toFixed(1);
    unit = "Mbps";
    elements.speedNumber.className =
      "speed-number " +
      (direction === "download" ? "downloading" : "uploading");
  }

  elements.speedNumber.textContent = displaySpeed;
  elements.speedUnit.textContent = unit;
}

export function updateProgress(progress) {
  if (!elements.progressRing) return;
  const clamped = Math.min(100, Math.max(0, progress));
  state.progress = clamped;
  if (!progressAnimationFrame) {
    progressAnimationFrame = requestAnimationFrame(animateProgressRing);
  }
  if (clamped >= 99.5 && clamped > visualProgress) {
    visualProgress = clamped;
  }
  if (visualProgress > state.progress) {
    visualProgress = state.progress;
  }
  if (elements.progressMeter) {
    const roundedProgress = Math.round(clamped);
    const nowMs = performance.now();
    const isBoundary = roundedProgress <= 0 || roundedProgress >= 100;
    const valueChanged = roundedProgress !== state.lastAriaProgressValue;
    const largeStep =
      Math.abs(roundedProgress - state.lastAriaProgressValue) >= 10;
    const pastThrottleWindow =
      nowMs - state.lastAriaProgressUpdateMs >=
      TEST_CONFIG.ARIA_PROGRESS_UPDATE_MS;
    if (valueChanged && (isBoundary || largeStep || pastThrottleWindow)) {
      if (typeof elements.progressMeter.value === "number") {
        elements.progressMeter.value = roundedProgress;
      }
      state.lastAriaProgressValue = roundedProgress;
      state.lastAriaProgressUpdateMs = nowMs;
    }
  }
}

function animateProgressRing() {
  progressAnimationFrame = 0;
  const targetProgress = state.progress;
  if (!Number.isFinite(targetProgress)) return;
  const diff = targetProgress - visualProgress;
  if (Math.abs(diff) > 0.1) {
    visualProgress += diff * 0.3;
  } else {
    visualProgress = targetProgress;
  }
  let offset = RING_CIRCUMFERENCE - (visualProgress / 100) * RING_CIRCUMFERENCE;
  if (visualProgress >= 99.5) {
    offset = -RING_END_OFFSET;
  }
  elements.progressRing.style.strokeDashoffset = offset;
  if (Math.abs(targetProgress - visualProgress) > 0.1) {
    progressAnimationFrame = requestAnimationFrame(animateProgressRing);
  }
}

export function resetProgress() {
  if (!elements.progressRing || !elements.speedNumber) return;
  state.progress = 0;
  visualProgress = 0;
  if (progressAnimationFrame) {
    cancelAnimationFrame(progressAnimationFrame);
    progressAnimationFrame = 0;
  }
  state.lastAriaProgressUpdateMs = 0;
  state.lastAriaProgressValue = 0;
  elements.progressRing.style.strokeDashoffset = RING_CIRCUMFERENCE;
  if (elements.progressMeter) {
    if (typeof elements.progressMeter.value === "number") {
      elements.progressMeter.value = 0;
    }
  }
  elements.speedNumber.textContent = "0";
}

export function updateTestType(text, className) {
  if (!elements.testType || !elements.progressRing || !elements.speedNumber)
    return;
  elements.testType.textContent = text;
  elements.progressRing.setAttribute(
    "class",
    "progress-ring-fill " + className,
  );
  elements.speedNumber.className = "speed-number " + className;
}

export function showState(stateName) {
  if (!elements.idleState || !elements.testingState || !elements.resultsState)
    return;
  elements.idleState.classList.add("hidden");
  elements.testingState.classList.add("hidden");
  elements.resultsState.classList.add("hidden");
  elements.testingState.setAttribute(
    "aria-busy",
    stateName === "testing" ? "true" : "false",
  );
  document.body.classList.toggle("results-view", stateName === "results");

  switch (stateName) {
    case "idle":
      elements.idleState.classList.remove("hidden");
      break;
    case "testing":
      elements.testingState.classList.remove("hidden");
      break;
    case "results":
      elements.resultsState.classList.remove("hidden");
      break;
  }
  focusStateAction(stateName);
}

export function showResults() {
  if (
    !elements.downloadResult ||
    !elements.uploadResult ||
    !elements.latencyResult ||
    !elements.jitterResult
  ) {
    return;
  }
  showState("results");

  const download = formatSpeed(state.downloadResult);
  const upload = formatSpeed(state.uploadResult);

  elements.downloadResult.textContent = download.value;
  elements.uploadResult.textContent = upload.value;

  const downloadUnit = document.querySelector(".result-primary .result-unit");
  const uploadUnit = document.querySelector(".result-secondary .result-unit");
  if (downloadUnit) downloadUnit.textContent = download.unit;
  if (uploadUnit) uploadUnit.textContent = upload.unit;

  elements.latencyResult.textContent = formatLatencyMs(state.latencyResult);
  elements.jitterResult.textContent = formatLatencyMs(state.jitterResult);

  const loadedLatency = Math.max(state.downloadLatency, state.uploadLatency);
  if (elements.loadedLatencyResult) {
    elements.loadedLatencyResult.textContent = formatLatencyMs(loadedLatency);
  }

  if (elements.bufferbloatResult) {
    const grade = computeBufferbloatGrade(state.latencyResult, loadedLatency);
    elements.bufferbloatResult.textContent = grade || "-";
  }

  updateNetworkDisplay();

  state.resultId = null;
  state.shareSavePromise = null;
  if (elements.shareBtn) {
    elements.shareBtn.classList.remove("hidden");
    elements.shareBtn.disabled = false;
    elements.shareBtn.textContent = "Share";
  }
}

export function showError(message, isError = true) {
  const { toastEl, messageEl, duration } = getToastElements(isError);
  if (!toastEl || !messageEl) return;
  clearToastTimer();
  hideError();
  messageEl.textContent = message;
  toastEl.classList.remove("hidden");
  toast.timer = setTimeout(hideError, duration);
}

export function hideError() {
  clearToastTimer();
  elements.errorToast?.classList.add("hidden");
  elements.successToast?.classList.add("hidden");
}

export function notifySettingsSaved() {
  if (elements.settingsModal?.open) {
    showError("Settings saved", false);
  }
}
