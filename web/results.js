/** Results page: load shared result by ID, render speed/latency/bufferbloat. */

import { formatSpeed, consumeErrorBody, fetchWithTimeout } from "./utils.js";

const RESULTS_TIMEOUT_MS = 20000;
const RESULT_ID_REGEX = /^[0-9a-zA-Z]{8}$/;

const loadingView = document.getElementById("loadingView");
const resultView = document.getElementById("resultView");
const errorView = document.getElementById("errorView");
const errorMessage = document.querySelector("#errorView .error-message");

function trimTrailingSlashes(value) {
  if (typeof value !== "string" || value.length === 0) return value;
  let end = value.length;
  while (end > 0 && value.codePointAt(end - 1) === 47) {
    end -= 1;
  }
  return value.slice(0, end);
}

function userError(message) {
  const err = new Error(message);
  err.userSafe = true;
  return err;
}

function resultErrorMessage(statusCode) {
  if (statusCode === 404) return "Result not found or has expired.";
  if (statusCode >= 500) return "Server error while loading result.";
  return "Unable to load result.";
}

function showError(message) {
  if (loadingView) loadingView.classList.add("hidden");
  if (resultView) resultView.classList.add("hidden");
  if (errorMessage && typeof message === "string" && message.trim() !== "") {
    errorMessage.textContent = message;
  }
  if (errorView) errorView.classList.remove("hidden");
}

function safeFixed(v, digits) {
  return typeof v === "number" && Number.isFinite(v) ? v.toFixed(digits) : "-";
}

function setText(el, text) {
  if (el) el.textContent = text;
}

function formatLatencyValue(v) {
  return typeof v === "number" && v > 0 ? safeFixed(v, 1) + " ms" : "-";
}

function updateServerDetails(d, refs) {
  if (!d.server_name) return;
  if (refs.serverLabelEl && refs.serverItemEl && refs.serverValueEl) {
    refs.serverLabelEl.classList.remove("hidden");
    refs.serverItemEl.classList.remove("hidden");
    refs.serverValueEl.textContent =
      typeof d.server_name === "string" ? d.server_name : String(d.server_name);
  }
}

function updateCreatedAt(createdAt, testedAtEl) {
  if (!createdAt || !testedAtEl) return;
  try {
    const date = new Date(createdAt);
    if (Number.isFinite(date.getTime())) {
      testedAtEl.textContent = date.toLocaleString();
    }
  } catch (err) {
    console.debug("results page: failed to parse created_at", err);
  }
}

async function loadResult(resultID) {
  const url = "/api/v1/results/" + resultID;
  const res = await fetchWithTimeout(url, {}, RESULTS_TIMEOUT_MS);
  if (!res.ok) {
    await consumeErrorBody(res);
    throw userError(resultErrorMessage(res.status));
  }
  return res.json();
}

function renderResult(d) {
  if (!d || typeof d !== "object") {
    showError("Invalid result payload.");
    return;
  }
  try {
    const downloadEl = document.getElementById("downloadResult");
    const uploadEl = document.getElementById("uploadResult");
    const latencyEl = document.getElementById("latencyResult");
    const jitterEl = document.getElementById("jitterResult");
    const loadedLatencyEl = document.getElementById("loadedLatencyResult");
    const bufferbloatEl = document.getElementById("bufferbloatResult");
    const ipv4El = document.getElementById("networkIPv4");
    const ipv6El = document.getElementById("networkIPv6");
    const serverLabelEl = document.getElementById("serverLabel");
    const serverItemEl = document.getElementById("serverItem");
    const serverValueEl = document.getElementById("serverValue");
    const testedAtEl = document.getElementById("testedAt");

    const dl = formatSpeed(
      Number.isFinite(d.download_mbps) ? d.download_mbps : 0,
    );
    const ul = formatSpeed(Number.isFinite(d.upload_mbps) ? d.upload_mbps : 0);

    setText(downloadEl, dl.value);
    setText(uploadEl, ul.value);

    const dlUnit = document.querySelector(".result-primary .result-unit");
    const ulUnit = document.querySelector(".result-secondary .result-unit");
    setText(dlUnit, dl.unit);
    setText(ulUnit, ul.unit);

    setText(latencyEl, formatLatencyValue(d.latency_ms));
    setText(jitterEl, formatLatencyValue(d.jitter_ms));
    setText(loadedLatencyEl, formatLatencyValue(d.loaded_latency_ms));
    if (bufferbloatEl) {
      bufferbloatEl.textContent =
        typeof d.bufferbloat_grade === "string" && d.bufferbloat_grade
          ? d.bufferbloat_grade
          : "-";
    }

    setText(ipv4El, d.ipv4 || "-");
    setText(ipv6El, d.ipv6 || "-");
    updateServerDetails(d, { serverLabelEl, serverItemEl, serverValueEl });
    updateCreatedAt(d.created_at, testedAtEl);

    document.title =
      "openByte — " +
      dl.value +
      " " +
      dl.unit +
      " / " +
      ul.value +
      " " +
      ul.unit;
  } catch (err) {
    console.error("results page: render failed", err);
    showError("Failed to render result.");
  }
}

if (!loadingView || !resultView || !errorView) {
  console.error("Results page missing required view elements");
} else {
  const normalizedPath = trimTrailingSlashes(globalThis.location.pathname);
  const parts = normalizedPath.split("/").filter(Boolean);
  const id = parts.at(-1);
  if (!id || !RESULT_ID_REGEX.test(id)) {
    showError("Result ID format is invalid.");
  } else {
    loadingView.classList.remove("hidden");
    resultView.classList.add("hidden");
    try {
      const data = await loadResult(id);
      loadingView.classList.add("hidden");
      resultView.classList.remove("hidden");
      renderResult(data);
    } catch (err) {
      console.error("Results fetch failed:", err);
      if (err?.name === "AbortError") {
        showError("Request timed out. Please try again.");
      } else if (err?.userSafe && err?.message) {
        showError(err.message);
      } else {
        showError("Unable to load result.");
      }
    }
  }
}
