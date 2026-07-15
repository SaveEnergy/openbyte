/** Result-state rendering for the main speed-test page. */

import { state, elements } from "./state.js";
import { t } from "./i18n.js";
import {
  formatLatency,
  formatLoadedLatencyAdvisory,
  formatSpeed,
  formatSpeedText,
} from "./presentation.js";
import { computeBufferbloatGrade } from "./utils.js";
import { updateNetworkDisplay } from "./network.js";

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

function renderAdvisory(loadedLatency) {
  const advisory = formatLoadedLatencyAdvisory({
    idleLatency: state.latencyResult,
    loadedLatency,
  });
  if (elements.resultsAdvisory) {
    elements.resultsAdvisory.textContent = advisory;
    elements.resultsAdvisory.classList.toggle("hidden", advisory === "");
  }
}

function announceResults(grade) {
  if (!elements.resultsAnnouncement) return;
  const download = formatSpeedText(state.downloadResult);
  const key = grade ? "announcement.completeWithGrade" : "announcement.complete";
  elements.resultsAnnouncement.textContent = t(key, {
    download,
    upload: formatSpeedText(state.uploadResult),
    latency: formatLatency(state.latencyResult),
    grade,
  });
}

function renderShareButton() {
  if (!elements.shareBtn) return;
  elements.shareBtn.textContent = t(
    elements.shareBtn.disabled ? "share.preparing" : "action.share",
  );
}

export function renderResultsContent() {
  if (
    !elements.downloadResult ||
    !elements.uploadResult ||
    !elements.latencyResult ||
    !elements.jitterResult
  ) {
    return;
  }

  const download = formatSpeed(state.downloadResult);
  elements.downloadResult.textContent = download.value;

  const downloadUnit = document.querySelector(".result-primary .result-unit");
  const uploadUnit = document.querySelector(".result-secondary .result-unit");
  if (downloadUnit) downloadUnit.textContent = download.unit;

  const upload = formatSpeed(state.uploadResult);
  elements.uploadResult.textContent = upload.value;
  if (uploadUnit) uploadUnit.textContent = upload.unit;

  elements.latencyResult.textContent = formatLatency(state.latencyResult);
  elements.jitterResult.textContent = formatLatency(state.jitterResult);

  const loadedLatency = Math.max(state.downloadLatency, state.uploadLatency);
  if (elements.loadedLatencyResult) {
    elements.loadedLatencyResult.textContent = formatLatency(loadedLatency);
  }

  const grade = computeBufferbloatGrade(state.latencyResult, loadedLatency);
  renderBufferbloat(grade);
  renderAdvisory(loadedLatency);
  announceResults(grade);

  updateNetworkDisplay();
  renderShareButton();
}

export function enterResults() {
  state.resultId = null;
  state.shareSavePromise = null;
  if (elements.shareBtn) {
    elements.shareBtn.classList.remove("hidden");
    elements.shareBtn.disabled = false;
  }
  renderResultsContent();
}
