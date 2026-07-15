/** Speed test run lifecycle (isolates openbyte.js from speedtest + ui state transitions). */

import { state, elements, getApiBase } from "./state.js";
import { computeBufferbloatGrade } from "./utils.js";
import {
  showState,
  showResults,
  showError,
  hideError,
  resetProgress,
  updateTestType,
  resetPhaseSteps,
  setActivePhaseStep,
  setPhaseStepValue,
  formatLatencyMs,
  formatSpeedText,
} from "./ui.js";
import { measureLatency, runDirectionPhase } from "./speedtest.js";
import { resolveServerName } from "./network.js";
import { saveHistoryEntry } from "./history.js";

function clearRunResults() {
  state.downloadResult = 0;
  state.uploadResult = 0;
  state.latencyResult = null;
  state.jitterResult = null;
  state.downloadLatency = 0;
  state.uploadLatency = 0;
  state.partialCancelRequested = false;
  state.lastResultPartial = false;
}

function finishAbortedRun() {
  if (state.partialCancelRequested && state.downloadResult > 0) {
    state.partialCancelRequested = false;
    state.phase = "results";
    showResults({ partial: true });
  } else {
    resetToIdle();
  }
}

function recordRunInHistory() {
  saveHistoryEntry({
    ts: Date.now(),
    down: state.downloadResult,
    up: state.uploadResult,
    latency: Number.isFinite(state.latencyResult) ? state.latencyResult : 0,
    grade:
      computeBufferbloatGrade(
        state.latencyResult,
        Math.max(state.downloadLatency, state.uploadLatency),
      ) || "",
  });
}

export async function startTest() {
  if (state.isRunning) {
    showError("Test already in progress");
    return;
  }

  state.isRunning = true;
  state.abortController = new AbortController();
  const signal = state.abortController.signal;
  clearRunResults();

  try {
    state.phase = "latency";
    resetProgress();
    resetPhaseSteps();
    setActivePhaseStep("ping");
    updateTestType("↔ Ping", "measuring");
    showState("testing");
    state.latencyResult = await measureLatency(signal);

    if (signal.aborted) {
      finishAbortedRun();
      return;
    }

    setPhaseStepValue("ping", formatLatencyMs(state.latencyResult));
    setActivePhaseStep("download");
    state.downloadResult = await runDirectionPhase(
      signal,
      "download",
      "↓ Download",
      "downloading",
      "download",
    );

    if (signal.aborted) {
      finishAbortedRun();
      return;
    }

    setPhaseStepValue("download", formatSpeedText(state.downloadResult));
    setActivePhaseStep("upload");
    state.uploadResult = await runDirectionPhase(
      signal,
      "upload",
      "↑ Upload",
      "uploading",
      "upload",
    );

    if (signal.aborted) {
      finishAbortedRun();
      return;
    }

    setPhaseStepValue("upload", formatSpeedText(state.uploadResult));
    state.phase = "results";
    recordRunInHistory();
    showResults();
  } catch (e) {
    if (e.name === "AbortError") {
      finishAbortedRun();
    } else {
      console.error("Test failed:", e);
      // Reset first: resetToIdle clears toasts, so the error must be shown after.
      if (state.abortController?.signal === signal) {
        resetToIdle();
      }
      showError(e.message || "Test failed");
    }
  } finally {
    if (state.abortController?.signal === signal) {
      state.isRunning = false;
      state.abortController = null;
    }
  }
}

export function cancelTest() {
  if (state.abortController) {
    state.abortController.abort();
  }
  state.isRunning = false;
}

/**
 * Cancel button: keep already-measured latency and download figures when the
 * download phase finished; otherwise there is nothing worth showing.
 */
export function handleCancel() {
  if (state.isRunning && state.downloadResult > 0) {
    state.partialCancelRequested = true;
    cancelTest();
    return;
  }
  resetToIdle();
}

export function resetToIdle() {
  cancelTest();

  state.phase = "idle";
  state.currentSpeed = 0;
  state.progress = 0;
  state.resultId = null;
  state.shareSavePromise = null;
  clearRunResults();
  if (elements.shareBtn) {
    elements.shareBtn.classList.add("hidden");
    elements.shareBtn.disabled = false;
    elements.shareBtn.textContent = "Share";
  }
  if (elements.partialNotice) {
    elements.partialNotice.classList.add("hidden");
  }

  resetProgress();
  showState("idle");
  hideError();
}

export async function saveAndEnableShare() {
  if (state.resultId) return state.resultId;
  if (state.shareSavePromise) return state.shareSavePromise;

  const loadedLat = Math.max(state.downloadLatency, state.uploadLatency);
  const bbGrade = computeBufferbloatGrade(state.latencyResult, loadedLat) || "";

  state.shareSavePromise = (async () => {
    const res = await fetch(`${getApiBase()}/results`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        download_mbps: state.downloadResult,
        upload_mbps: state.uploadResult,
        latency_ms: state.latencyResult || 0,
        jitter_ms: state.jitterResult || 0,
        loaded_latency_ms: loadedLat,
        bufferbloat_grade: bbGrade,
        ipv4: state.networkInfo.ipv4 || "",
        ipv6: state.networkInfo.ipv6 || "",
        server_name: resolveServerName(),
      }),
    });
    if (!res.ok) {
      await res.text().catch(() => {});
      throw new Error(`share save failed: HTTP ${res.status}`);
    }
    const data = await res.json();
    if (state.phase !== "results") return null;
    if (typeof data?.id !== "string" || data.id.length === 0) {
      throw new Error("share save failed: invalid response");
    }
    state.resultId = data.id;
    return state.resultId;
  })();

  try {
    return await state.shareSavePromise;
  } finally {
    state.shareSavePromise = null;
  }
}

function copyShareUrl() {
  if (!state.resultId) return;
  const url = globalThis.location.origin + "/results/" + state.resultId;
  if (navigator.clipboard?.writeText) {
    navigator.clipboard
      .writeText(url)
      .then(() => {
        showError("Link copied to clipboard", false);
      })
      .catch(() => {
        promptShareUrl(url);
      });
  } else {
    promptShareUrl(url);
  }
}

/** One tap: save the result (first time) and hand over the link. */
export async function handleShare() {
  if (state.phase !== "results" || state.lastResultPartial) return;

  if (state.resultId) {
    copyShareUrl();
    return;
  }

  if (elements.shareBtn) {
    elements.shareBtn.disabled = true;
    elements.shareBtn.textContent = "Preparing...";
  }
  try {
    await saveAndEnableShare();
    if (state.phase === "results") {
      copyShareUrl();
    }
  } catch (err) {
    console.debug("Share save unavailable:", err);
    showError("Unable to create share link right now");
  } finally {
    if (elements.shareBtn && state.phase === "results") {
      elements.shareBtn.disabled = false;
      elements.shareBtn.textContent = "Share";
    }
  }
}

export function promptShareUrl(url) {
  if (navigator.share) {
    navigator
      .share({ title: "openByte Speed Test Result", url })
      .catch(() => {});
  } else {
    globalThis.prompt("Copy this link:", url);
  }
}
