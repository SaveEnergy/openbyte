/** DOM updates: progress, speed display, state views, toast. */

import { state, elements, TEST_CONFIG, toast } from "./state.js";
import { computeBufferbloatGrade, formatSpeed } from "./utils.js";
import { updateNetworkDisplay } from "./network.js";

function focusStateAction(stateName) {
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

function setInstrumentActivity(speed, direction) {
  if (!elements.testingState) return;

  const normalized =
    direction === "latency"
      ? Math.max(0.1, Math.min(1, 1 - speed / 180))
      : Math.max(0.08, Math.min(1, Math.log10(speed + 1) / 3));

  const glowSize = 18 + normalized * 34;
  elements.testingState.style.setProperty(
    "--instrument-glow-size",
    `${glowSize.toFixed(0)}px`,
  );
  elements.testingState.style.setProperty(
    "--instrument-opacity",
    (0.32 + normalized * 0.3).toFixed(2),
  );
}

export function updateSpeed(speed, direction) {
  if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
    speed = 0;
  if (!elements.speedNumber || !elements.speedUnit) return;
  state.currentSpeed = speed;
  setInstrumentActivity(speed, direction);

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
  const clamped = Math.min(100, Math.max(0, progress));
  state.progress = clamped;

  if (elements.progressMeter) {
    elements.progressMeter.removeAttribute("value");
  }
}

export function resetProgress() {
  if (!elements.speedNumber) return;
  state.progress = 0;
  if (elements.progressMeter) {
    elements.progressMeter.removeAttribute("value");
    elements.progressMeter.textContent = "Measuring network";
  }
  if (elements.testingState) {
    elements.testingState.style.removeProperty("--instrument-glow-size");
    elements.testingState.style.removeProperty("--instrument-opacity");
  }
  elements.speedNumber.textContent = "0";
}

export function updateTestType(text, className) {
  if (!elements.testType || !elements.progressRing || !elements.speedNumber)
    return;
  elements.testType.textContent = text;
  elements.speedNumber.textContent = "0";
  if (elements.speedUnit) {
    elements.speedUnit.textContent = className === "measuring" ? "ms" : "Mbps";
  }
  if (elements.progressMeter) {
    elements.progressMeter.setAttribute("aria-label", `${text} in progress`);
    elements.progressMeter.textContent = text;
  }
  if (elements.testingState) {
    elements.testingState.dataset.phase = className;
  }
  elements.progressRing.setAttribute(
    "class",
    "instrument-ring-arc " + className,
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
