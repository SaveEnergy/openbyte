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
import { renderHistory } from "./history.js";

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

function renderAdvisory(partial, loadedLatency) {
  const advisory = formatLoadedLatencyAdvisory({
    idleLatency: state.latencyResult,
    loadedLatency,
    partial,
  });
  if (elements.resultsAdvisory) {
    elements.resultsAdvisory.textContent = advisory;
    elements.resultsAdvisory.classList.toggle("hidden", advisory === "");
  }
}

function announceResults(partial, grade) {
  if (!elements.resultsAnnouncement) return;
  const download = formatSpeedText(state.downloadResult);
  let key = partial ? "announcement.partial" : "announcement.complete";
  if (grade) key += "WithGrade";
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
  const partial = state.lastResultPartial;
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

  if (partial) {
    elements.uploadResult.textContent = "—";
    if (uploadUnit) uploadUnit.textContent = t("result.notMeasured");
  } else {
    const upload = formatSpeed(state.uploadResult);
    elements.uploadResult.textContent = upload.value;
    if (uploadUnit) uploadUnit.textContent = upload.unit;
  }

  elements.latencyResult.textContent = formatLatency(state.latencyResult);
  elements.jitterResult.textContent = formatLatency(state.jitterResult);

  const loadedLatency = Math.max(state.downloadLatency, state.uploadLatency);
  if (elements.loadedLatencyResult) {
    elements.loadedLatencyResult.textContent = formatLatency(loadedLatency);
  }
  if (elements.loadedLatencyLabel) {
    elements.loadedLatencyLabel.textContent = t(
      partial ? "metric.downloadLatency" : "metric.loadedLatency",
    );
  }

  const grade = partial
    ? null
    : computeBufferbloatGrade(state.latencyResult, loadedLatency);
  elements.bufferbloatStat?.classList.toggle("hidden", partial);
  elements.statsHelp?.classList.toggle("hidden", partial);
  renderBufferbloat(grade);
  renderAdvisory(partial, loadedLatency);
  announceResults(partial, grade);

  elements.partialNotice?.classList.toggle("hidden", !partial);
  updateNetworkDisplay();
  renderHistory(elements.historyList, elements.historySection);
  renderShareButton();
}

export function enterResults(partial) {
  state.lastResultPartial = partial;
  state.resultId = null;
  state.shareSavePromise = null;
  if (elements.shareBtn) {
    // Partial runs have no upload figure, so a saved share would be misleading.
    elements.shareBtn.classList.toggle("hidden", partial);
    elements.shareBtn.disabled = false;
  }
  renderResultsContent();
}
