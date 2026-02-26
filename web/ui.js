/** DOM updates: progress, speed display, state views, toast. */

import {
  state,
  elements,
  RING_CIRCUMFERENCE,
  RING_END_OFFSET,
  TEST_CONFIG,
  toast,
} from "./state.js";
import { computeBufferbloatGrade } from "./utils.js";
import { updateNetworkDisplay } from "./network.js";

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
  state.progress = progress;
  let offset = RING_CIRCUMFERENCE - (progress / 100) * RING_CIRCUMFERENCE;
  if (progress >= 99.5) {
    offset = -RING_END_OFFSET;
  }
  elements.progressRing.style.strokeDashoffset = offset;
  if (elements.progressMeter) {
    const roundedProgress = Math.round(progress);
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

export function resetProgress() {
  if (!elements.progressRing || !elements.speedNumber) return;
  state.progress = 0;
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

  const formatSpeedWithUnit = (speed) => {
    if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
      speed = 0;
    if (speed >= 1000) {
      return { value: (speed / 1000).toFixed(2), unit: "Gbps" };
    }
    return { value: speed.toFixed(1), unit: "Mbps" };
  };

  const download = formatSpeedWithUnit(state.downloadResult);
  const upload = formatSpeedWithUnit(state.uploadResult);

  elements.downloadResult.textContent = download.value;
  elements.uploadResult.textContent = upload.value;

  const downloadUnit = document.querySelector(".result-primary .result-unit");
  const uploadUnit = document.querySelector(".result-secondary .result-unit");
  if (downloadUnit) downloadUnit.textContent = download.unit;
  if (uploadUnit) uploadUnit.textContent = upload.unit;

  elements.latencyResult.textContent =
    state.latencyResult != null ? `${state.latencyResult.toFixed(1)} ms` : "-";
  elements.jitterResult.textContent =
    state.jitterResult != null ? `${state.jitterResult.toFixed(1)} ms` : "-";

  const loadedLatency = Math.max(state.downloadLatency, state.uploadLatency);
  if (elements.loadedLatencyResult) {
    elements.loadedLatencyResult.textContent =
      loadedLatency > 0 ? `${loadedLatency.toFixed(1)} ms` : "-";
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
  if (!elements.errorToast || !elements.errorMessage) return;
  elements.errorMessage.textContent = message;
  const icon = elements.errorToast.querySelector(".toast-icon");
  if (toast.timer) {
    clearTimeout(toast.timer);
    toast.timer = null;
  }
  if (isError) {
    if (icon) icon.textContent = "⚠";
    elements.errorToast.classList.remove("hidden");
    elements.errorToast.style.background = "";
    toast.timer = setTimeout(hideError, TEST_CONFIG.TOAST_ERROR_MS);
  } else {
    if (icon) icon.textContent = "✓";
    elements.errorToast.classList.remove("hidden");
    elements.errorToast.style.background = "var(--accent-primary)";
    toast.timer = setTimeout(() => {
      hideError();
      elements.errorToast.style.background = "";
    }, TEST_CONFIG.TOAST_SUCCESS_MS);
  }
}

export function hideError() {
  if (!elements.errorToast) return;
  elements.errorToast.classList.add("hidden");
}

export function notifySettingsSaved() {
  if (elements.settingsModal?.open) {
    showError("Settings saved", false);
  }
}
