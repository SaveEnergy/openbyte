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

function isCurrentRun(signal) {
  return state.abortController?.signal === signal;
}

function finishAbortedRun(signal) {
  if (!isCurrentRun(signal)) return;
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
  if (!state.serverOnline) {
    showError("Server is not ready yet");
    return;
  }

  state.isRunning = true;
  state.abortController = new AbortController();
  const signal = state.abortController.signal;
  state.runGeneration += 1;
  clearRunResults();

  try {
    state.phase = "latency";
    resetProgress();
    resetPhaseSteps();
    setActivePhaseStep("ping");
    updateTestType("↔ Ping", "measuring");
    showState("testing");
    const latency = await measureLatency(signal);

    if (!isCurrentRun(signal)) return;
    if (signal.aborted) {
      finishAbortedRun(signal);
      return;
    }
    state.latencyResult = latency.value;
    state.jitterResult = latency.jitter;

    setPhaseStepValue("ping", formatLatencyMs(state.latencyResult));
    setActivePhaseStep("download");
    const downloadResult = await runDirectionPhase(
      signal,
      "download",
      "↓ Download",
      "downloading",
      "download",
    );

    if (!isCurrentRun(signal)) return;
    if (signal.aborted) {
      finishAbortedRun(signal);
      return;
    }
    state.downloadResult = downloadResult;

    setPhaseStepValue("download", formatSpeedText(state.downloadResult));
    setActivePhaseStep("upload");
    const uploadResult = await runDirectionPhase(
      signal,
      "upload",
      "↑ Upload",
      "uploading",
      "upload",
    );

    if (!isCurrentRun(signal)) return;
    if (signal.aborted) {
      finishAbortedRun(signal);
      return;
    }
    state.uploadResult = uploadResult;

    setPhaseStepValue("upload", formatSpeedText(state.uploadResult));
    state.phase = "results";
    recordRunInHistory();
    showResults();
  } catch (e) {
    if (!isCurrentRun(signal)) return;
    if (e.name === "AbortError") {
      finishAbortedRun(signal);
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
  state.abortController = null;

  state.runGeneration += 1;
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

  const generation = state.runGeneration;
  const loadedLat = Math.max(state.downloadLatency, state.uploadLatency);
  const bbGrade = computeBufferbloatGrade(state.latencyResult, loadedLat) || "";

  const savePromise = (async () => {
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
    if (state.phase !== "results" || state.runGeneration !== generation) {
      return null;
    }
    if (typeof data?.id !== "string" || data.id.length === 0) {
      throw new Error("share save failed: invalid response");
    }
    state.resultId = data.id;
    return state.resultId;
  })();
  state.shareSavePromise = savePromise;

  try {
    return await savePromise;
  } finally {
    if (state.shareSavePromise === savePromise) {
      state.shareSavePromise = null;
    }
  }
}

function copyShareUrl(resultId = state.resultId) {
  if (!resultId) return;
  const url = globalThis.location.origin + "/results/" + resultId;
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

  const generation = state.runGeneration;
  if (state.resultId) {
    copyShareUrl();
    return;
  }

  if (elements.shareBtn) {
    elements.shareBtn.disabled = true;
    elements.shareBtn.textContent = "Preparing...";
  }
  try {
    const resultId = await saveAndEnableShare();
    if (
      resultId &&
      state.phase === "results" &&
      state.runGeneration === generation
    ) {
      copyShareUrl(resultId);
    }
  } catch (err) {
    console.debug("Share save unavailable:", err);
    if (state.phase === "results" && state.runGeneration === generation) {
      showError("Unable to create share link right now");
    }
  } finally {
    if (
      elements.shareBtn &&
      state.phase === "results" &&
      state.runGeneration === generation
    ) {
      elements.shareBtn.disabled = false;
      elements.shareBtn.textContent = "Share";
    }
  }
}

export function promptShareUrl(url) {
  if (navigator.share) {
    navigator
      .share({ title: "openByte Speed Test Result", url })
      .catch((err) => {
        if (err?.name !== "AbortError") {
          globalThis.prompt("Copy this link:", url);
        }
      });
  } else {
    globalThis.prompt("Copy this link:", url);
  }
}
