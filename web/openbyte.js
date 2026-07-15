/** Main entry: wiring, network detection, share/init, and test lifecycle. */

import {
  state,
  elements,
  getApiBase,
  initElements,
  TEST_CONFIG,
} from "./state.js";
import { t } from "./i18n.js";
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
import {
  detectNetworkInfo,
  resolveServerName,
  checkServer,
  renderNetworkState,
} from "./network.js";
import { saveHistoryEntry } from "./history.js";

function testErrorKey(error) {
  const code = error?.code;
  return typeof code === "string" && t(code) ? code : "error.testFailed";
}

function clearRunResults() {
  state.downloadResult = 0;
  state.uploadResult = 0;
  state.latencyResult = null;
  state.jitterResult = null;
  state.downloadLatency = 0;
  state.uploadLatency = 0;
}

function isCurrentRun(signal) {
  return state.abortController?.signal === signal;
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
    showError("error.testInProgress");
    return;
  }
  if (!state.serverOnline) {
    showError("error.serverNotReady");
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
    updateTestType("test.phase.ping", "measuring", { icon: "↔" });
    showState("testing");
    const latency = await measureLatency(signal);

    if (!isCurrentRun(signal)) return;
    if (signal.aborted) {
      resetToIdle();
      return;
    }
    state.latencyResult = latency.value;
    state.jitterResult = latency.jitter;

    const downloadResult = await runDirectionPhase(
      signal,
      "download",
      "test.phase.download",
      "downloading",
      "download",
    );

    if (!isCurrentRun(signal)) return;
    if (signal.aborted) {
      resetToIdle();
      return;
    }
    state.downloadResult = downloadResult;

    const uploadResult = await runDirectionPhase(
      signal,
      "upload",
      "test.phase.upload",
      "uploading",
      "upload",
    );

    if (!isCurrentRun(signal)) return;
    if (signal.aborted) {
      resetToIdle();
      return;
    }
    state.uploadResult = uploadResult;

    state.phase = "results";
    recordRunInHistory();
    showResults();
  } catch (e) {
    if (!isCurrentRun(signal)) return;
    if (e.name === "AbortError") {
      resetToIdle();
    } else {
      console.error("Test failed:", e);
      // Reset first: resetToIdle clears toasts, so the error must be shown after.
      if (state.abortController?.signal === signal) {
        resetToIdle();
      }
      showError(testErrorKey(e));
    }
  } finally {
    if (state.abortController?.signal === signal) {
      state.isRunning = false;
      state.abortController = null;
    }
  }
}

export function resetToIdle() {
  state.abortController?.abort();
  state.isRunning = false;
  state.abortController = null;

  state.runGeneration += 1;
  state.phase = "idle";
  state.currentSpeed = 0;
  state.progress = 0;
  state.resultId = null;
  state.shareSavePromise = null;
  state.testType = null;
  clearRunResults();
  if (elements.shareBtn) {
    elements.shareBtn.classList.add("hidden");
    elements.shareBtn.disabled = false;
    elements.shareBtn.textContent = t("action.share");
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
        showError("share.copied", false);
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
  if (state.phase !== "results") return;

  const generation = state.runGeneration;
  if (state.resultId) {
    copyShareUrl();
    return;
  }

  if (elements.shareBtn) {
    elements.shareBtn.disabled = true;
    elements.shareBtn.textContent = t("share.preparing");
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
      showError("share.unavailable");
    }
  } finally {
    if (
      elements.shareBtn &&
      state.phase === "results" &&
      state.runGeneration === generation
    ) {
      elements.shareBtn.disabled = false;
      elements.shareBtn.textContent = t("action.share");
    }
  }
}

export function promptShareUrl(url) {
  if (navigator.share) {
    navigator
      .share({ title: t("share.nativeTitle"), url })
      .catch((err) => {
        if (err?.name !== "AbortError") {
          globalThis.prompt(t("share.copyPrompt"), url);
        }
      });
  } else {
    globalThis.prompt(t("share.copyPrompt"), url);
  }
}

function bindEvents() {
  if (!elements.startBtn || !elements.restartBtn) {
    console.warn("Core UI elements missing; skipping event binding");
    return;
  }
  elements.startBtn.addEventListener("click", startTest);
  elements.restartBtn.addEventListener("click", resetToIdle);
  elements.cancelBtn?.addEventListener("click", resetToIdle);
  elements.shareBtn?.addEventListener("click", handleShare);
}

function init() {
  initElements();
  renderNetworkState();
  bindEvents();
  detectNetworkInfo();
  setInterval(() => {
    if (!state.isRunning && state.phase === "idle") checkServer();
  }, TEST_CONFIG.SERVER_RECHECK_MS);
}

init();
