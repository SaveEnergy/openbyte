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
} from "./ui.js";
import { measureLatency, runDirectionPhase } from "./speedtest.js";
import { resolveServerName } from "./network.js";

export async function startTest() {
  if (state.isRunning) {
    showError("Test already in progress");
    return;
  }

  state.isRunning = true;
  state.abortController = new AbortController();
  const signal = state.abortController.signal;

  try {
    state.phase = "latency";
    resetProgress();
    updateTestType("↔ Ping", "measuring");
    showState("testing");
    state.latencyResult = await measureLatency(signal);

    if (signal.aborted) return;

    state.downloadResult = await runDirectionPhase(
      signal,
      "download",
      "↓ Download",
      "downloading",
      "download",
    );

    if (signal.aborted) return;

    state.uploadResult = await runDirectionPhase(
      signal,
      "upload",
      "↑ Upload",
      "uploading",
      "upload",
    );

    if (signal.aborted) return;

    state.phase = "results";
    showResults();
  } catch (e) {
    console.error("Test failed:", e);
    if (e.name !== "AbortError") {
      const message = e.message || "Test failed";
      showError(message);
    }
    if (state.abortController?.signal === signal) {
      resetToIdle();
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

export function resetToIdle() {
  cancelTest();

  state.phase = "idle";
  state.currentSpeed = 0;
  state.progress = 0;
  state.downloadResult = 0;
  state.uploadResult = 0;
  state.latencyResult = null;
  state.jitterResult = null;
  state.diagnostics = null;
  state.downloadLatency = 0;
  state.uploadLatency = 0;
  state.resultId = null;
  state.shareSavePromise = null;
  if (elements.shareBtn) {
    elements.shareBtn.classList.add("hidden");
    elements.shareBtn.disabled = false;
    elements.shareBtn.textContent = "Share";
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
        ...(state.diagnostics && { diagnostics: state.diagnostics }),
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

export async function handleShare() {
  if (state.phase !== "results") return;

  if (!state.resultId) {
    if (elements.shareBtn) {
      elements.shareBtn.disabled = true;
      elements.shareBtn.textContent = "Preparing...";
    }
    saveAndEnableShare()
      .then(() => {
        if (elements.shareBtn && state.phase === "results") {
          elements.shareBtn.disabled = false;
          elements.shareBtn.textContent = "Share";
        }
        showError("Share link ready — tap Share again", false);
      })
      .catch((err) => {
        console.debug("Share save unavailable:", err);
        if (elements.shareBtn && state.phase === "results") {
          elements.shareBtn.disabled = false;
          elements.shareBtn.textContent = "Share";
        }
        showError("Unable to create share link right now");
      });
    return;
  }

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

export function promptShareUrl(url) {
  if (navigator.share) {
    navigator
      .share({ title: "openByte Speed Test Result", url })
      .catch(() => {});
  } else {
    globalThis.prompt("Copy this link:", url);
  }
}
