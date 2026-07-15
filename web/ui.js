/** DOM updates: progress, speed display, state views, toast. */

import { state, elements, TEST_CONFIG, toast } from "./state.js";
import {
  computeBufferbloatGrade,
  computeConnectionVerdict,
  formatSpeed,
} from "./utils.js";
import { updateNetworkDisplay } from "./network.js";
import { renderHistory } from "./history.js";

/** Circumference of the progress ring arc (r=90 in the 200x200 viewBox). */
const RING_CIRCUMFERENCE = 2 * Math.PI * 90;

const PHASE_ORDER = ["ping", "download", "upload"];

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

export function formatLatencyMs(val) {
  if (typeof val === "number" && Number.isFinite(val) && val > 0) {
    return `${val.toFixed(1)} ms`;
  }
  return "-";
}

export function formatSpeedText(mbps) {
  const formatted = formatSpeed(mbps);
  return `${formatted.value} ${formatted.unit}`;
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

/* ---- Live sparkline ---- */

const sparkline = { samples: [], color: "#00d4aa" };

function sparklineColorFor(direction) {
  return direction === "upload" ? "#667eea" : "#00d4aa";
}

function ensureSparklineScale(canvas) {
  if (canvas.dataset.scaled) return;
  canvas.dataset.scaled = "1";
  const ratio = globalThis.devicePixelRatio || 1;
  if (ratio === 1) return;
  const cssWidth = canvas.width;
  const cssHeight = canvas.height;
  canvas.style.width = `${cssWidth}px`;
  canvas.style.height = `${cssHeight}px`;
  canvas.width = Math.round(cssWidth * ratio);
  canvas.height = Math.round(cssHeight * ratio);
}

function drawSparkline() {
  const canvas = elements.speedSparkline;
  if (!canvas || typeof canvas.getContext !== "function") return;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ensureSparklineScale(canvas);

  const { width, height } = canvas;
  const ratio = globalThis.devicePixelRatio || 1;
  ctx.clearRect(0, 0, width, height);

  const samples = sparkline.samples;
  if (samples.length < 2) return;
  const max = Math.max(...samples);
  if (max <= 0) return;

  const padding = 2 * ratio;
  const stepX = (width - 2 * padding) / (samples.length - 1);
  const usableHeight = height - 2 * padding;

  ctx.beginPath();
  for (let i = 0; i < samples.length; i++) {
    const x = padding + i * stepX;
    const y = height - padding - (samples[i] / max) * usableHeight;
    if (i === 0) {
      ctx.moveTo(x, y);
    } else {
      ctx.lineTo(x, y);
    }
  }
  ctx.strokeStyle = sparkline.color;
  ctx.lineWidth = 2 * ratio;
  ctx.lineJoin = "round";
  ctx.lineCap = "round";
  ctx.stroke();

  ctx.lineTo(padding + (samples.length - 1) * stepX, height);
  ctx.lineTo(padding, height);
  ctx.closePath();
  ctx.globalAlpha = 0.12;
  ctx.fillStyle = sparkline.color;
  ctx.fill();
  ctx.globalAlpha = 1;
}

export function resetSparkline() {
  sparkline.samples = [];
  drawSparkline();
}

function recordSparklinePoint(speed, direction) {
  if (direction !== "download" && direction !== "upload") return;
  sparkline.color = sparklineColorFor(direction);
  sparkline.samples.push(speed);
  if (sparkline.samples.length > TEST_CONFIG.SPARKLINE_MAX_POINTS) {
    sparkline.samples.shift();
  }
  drawSparkline();
}

/* ---- Phase stepper ---- */

export function resetPhaseSteps() {
  for (const phase of PHASE_ORDER) {
    const step = elements.phaseSteps?.[phase];
    const value = elements.phaseValues?.[phase];
    if (step) step.dataset.status = "pending";
    if (value) value.textContent = "";
  }
}

export function setActivePhaseStep(activePhase) {
  const activeIndex = PHASE_ORDER.indexOf(activePhase);
  if (activeIndex === -1) return;
  PHASE_ORDER.forEach((phase, index) => {
    const step = elements.phaseSteps?.[phase];
    if (!step) return;
    if (index < activeIndex) {
      step.dataset.status = "done";
    } else if (index === activeIndex) {
      step.dataset.status = "active";
    } else {
      step.dataset.status = "pending";
    }
  });
}

export function setPhaseStepValue(phase, text) {
  const step = elements.phaseSteps?.[phase];
  const value = elements.phaseValues?.[phase];
  if (step) step.dataset.status = "done";
  if (value) value.textContent = text;
}

/* ---- Speed and progress display ---- */

export function updateSpeed(speed, direction) {
  if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
    speed = 0;
  if (!elements.speedNumber || !elements.speedUnit) return;
  state.currentSpeed = speed;
  setInstrumentActivity(speed, direction);
  recordSparklinePoint(speed, direction);

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
    elements.progressMeter.textContent = "Measuring network";
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

export function updateTestType(text, className) {
  if (!elements.testType || !elements.speedNumber) return;
  elements.testType.textContent = text;
  if (elements.progressMeter) {
    elements.progressMeter.setAttribute("aria-label", `${text} in progress`);
    elements.progressMeter.textContent = text;
  }

  const phaseChanged = elements.testingState?.dataset.phase !== className;
  if (phaseChanged) {
    // Only reset the live displays on a real phase transition; ramp windows
    // within a phase re-label without wiping the current reading.
    elements.speedNumber.textContent = "0";
    if (elements.speedUnit) {
      elements.speedUnit.textContent =
        className === "measuring" ? "ms" : "Mbps";
    }
    resetSparkline();
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

/* ---- Results ---- */

function bufferbloatBadgeClass(grade) {
  if (grade === "A+" || grade === "A") return "bb-good";
  if (grade === "B" || grade === "C") return "bb-mid";
  if (grade === "D" || grade === "F") return "bb-bad";
  return "";
}

function renderBufferbloat(grade) {
  const el = elements.bufferbloatResult;
  if (!el) return;
  el.classList.remove("bb-good", "bb-mid", "bb-bad");
  el.textContent = grade || "-";
  const badgeClass = bufferbloatBadgeClass(grade);
  if (badgeClass) el.classList.add(badgeClass);
}

function renderVerdict(partial, loadedLatency) {
  if (!elements.resultsVerdict) return;
  const verdict = computeConnectionVerdict({
    download: state.downloadResult,
    upload: state.uploadResult,
    idleLatency: state.latencyResult,
    loadedLatency,
    partial,
  });
  elements.resultsVerdict.textContent = verdict;
  elements.resultsVerdict.classList.toggle("hidden", verdict === "");
}

function announceResults(partial, grade) {
  if (!elements.resultsAnnouncement) return;
  const download = formatSpeedText(state.downloadResult);
  const parts = [`Speed test ${partial ? "cancelled early" : "complete"}.`];
  parts.push(`Download ${download}.`);
  if (!partial) {
    parts.push(`Upload ${formatSpeedText(state.uploadResult)}.`);
  }
  parts.push(`Latency ${formatLatencyMs(state.latencyResult)}.`);
  if (grade) {
    parts.push(`Bufferbloat grade ${grade}.`);
  }
  elements.resultsAnnouncement.textContent = parts.join(" ");
}

export function showResults(options = {}) {
  const partial = options.partial === true;
  if (
    !elements.downloadResult ||
    !elements.uploadResult ||
    !elements.latencyResult ||
    !elements.jitterResult
  ) {
    return;
  }
  state.lastResultPartial = partial;
  showState("results");

  const download = formatSpeed(state.downloadResult);

  elements.downloadResult.textContent = download.value;

  const downloadUnit = document.querySelector(".result-primary .result-unit");
  const uploadUnit = document.querySelector(".result-secondary .result-unit");
  if (downloadUnit) downloadUnit.textContent = download.unit;

  if (partial) {
    elements.uploadResult.textContent = "—";
    if (uploadUnit) uploadUnit.textContent = "not measured";
  } else {
    const upload = formatSpeed(state.uploadResult);
    elements.uploadResult.textContent = upload.value;
    if (uploadUnit) uploadUnit.textContent = upload.unit;
  }

  elements.latencyResult.textContent = formatLatencyMs(state.latencyResult);
  elements.jitterResult.textContent = formatLatencyMs(state.jitterResult);

  const loadedLatency = Math.max(state.downloadLatency, state.uploadLatency);
  if (elements.loadedLatencyResult) {
    elements.loadedLatencyResult.textContent = formatLatencyMs(loadedLatency);
  }

  const grade = computeBufferbloatGrade(state.latencyResult, loadedLatency);
  renderBufferbloat(grade);
  renderVerdict(partial, loadedLatency);
  announceResults(partial, grade);

  if (elements.partialNotice) {
    elements.partialNotice.classList.toggle("hidden", !partial);
  }

  updateNetworkDisplay();
  renderHistory(elements.historyList, elements.historySection);

  state.resultId = null;
  state.shareSavePromise = null;
  if (elements.shareBtn) {
    // Partial runs have no upload figure, so a saved share would be misleading.
    elements.shareBtn.classList.toggle("hidden", partial);
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
