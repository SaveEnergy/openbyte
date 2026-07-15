/** DOM updates: progress, speed display, state views, toast. */

import { state, elements, TEST_CONFIG, toast } from "./state.js";
import { formatNumber, t } from "./i18n.js";
import { formatSpeed } from "./presentation.js";
import { enterResults } from "./ui-results.js";

/** Circumference of the progress ring arc (r=90 in the 200x200 viewBox). */
const RING_CIRCUMFERENCE = 2 * Math.PI * 90;

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

/* ---- Speed and progress display ---- */

export function updateSpeed(speed, direction) {
  if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
    speed = 0;
  if (!elements.speedNumber || !elements.speedUnit) return;
  state.currentSpeed = speed;
  setInstrumentActivity(speed, direction);

  let displaySpeed, unit;

  if (direction === "latency") {
    displaySpeed = formatNumber(speed, { maximumFractionDigits: 0 });
    unit = "ms";
    elements.speedNumber.className = "speed-number measuring";
  } else {
    ({ value: displaySpeed, unit } = formatSpeed(speed));
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
    elements.progressMeter.value = clamped;
  }
  if (elements.progressRing) {
    const arcLength = (clamped / 100) * RING_CIRCUMFERENCE;
    elements.progressRing.style.strokeDasharray = `${arcLength.toFixed(1)} ${RING_CIRCUMFERENCE.toFixed(1)}`;
  }
}

export function resetProgress() {
  if (!elements.speedNumber) return;
  if (elements.progressMeter) {
    elements.progressMeter.value = 0;
    elements.progressMeter.textContent = t("test.progressText");
    elements.progressMeter.setAttribute("aria-label", t("test.progressAria"));
  }
  if (elements.progressRing) {
    elements.progressRing.style.strokeDasharray = `0 ${RING_CIRCUMFERENCE.toFixed(1)}`;
  }
  if (elements.testingState) {
    elements.testingState.style.removeProperty("--instrument-glow-size");
    elements.testingState.style.removeProperty("--instrument-opacity");
  }
  elements.speedNumber.textContent = "0";
}

function renderTestType() {
  if (!elements.testType || !elements.speedNumber) return;
  const current = state.testType;
  if (!current) return;
  const phase = t(current.key);
  const text = `${current.icon || ""}${phase}`;
  elements.testType.textContent = text;
  if (elements.progressMeter) {
    elements.progressMeter.setAttribute(
      "aria-label",
      t("test.phaseInProgress", { phase }),
    );
    elements.progressMeter.textContent = text;
  }
}

export function updateTestType(key, className, options = {}) {
  if (!elements.testType || !elements.speedNumber) return;
  state.testType = {
    key,
    className,
    icon: options.icon ? `${options.icon} ` : "",
  };
  renderTestType();

  const phaseChanged = elements.testingState?.dataset.phase !== className;
  if (phaseChanged) {
    // Only reset the live displays on a real phase transition; ramp windows
    // within a phase re-label without wiping the current reading.
    elements.speedNumber.textContent = "0";
    if (elements.speedUnit) {
      elements.speedUnit.textContent =
        className === "measuring" ? "ms" : "Mbps";
    }
    if (elements.testingState) {
      elements.testingState.dataset.phase = className;
    }
    if (elements.progressRing) {
      elements.progressRing.setAttribute(
        "class",
        "instrument-ring-arc " + className,
      );
    }
    elements.speedNumber.className = "speed-number " + className;
  }
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
  showState("results");
  enterResults();
}

function renderToast() {
  if (!toast.key) return;
  const { messageEl } = getToastElements(toast.isError);
  if (messageEl) messageEl.textContent = t(toast.key, toast.variables);
}

export function showError(key, isError = true, variables = {}) {
  const { toastEl, messageEl, duration } = getToastElements(isError);
  if (!toastEl || !messageEl) return;
  clearToastTimer();
  hideError();
  toast.key = key;
  toast.variables = variables;
  toast.isError = isError;
  renderToast();
  toastEl.classList.remove("hidden");
  toast.timer = setTimeout(hideError, duration);
}

export function hideError() {
  clearToastTimer();
  elements.errorToast?.classList.add("hidden");
  elements.successToast?.classList.add("hidden");
}
