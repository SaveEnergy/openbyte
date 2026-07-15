/** Results page: load shared result by ID, render speed/latency/bufferbloat. */

import {
  consumeErrorBody,
  fetchWithTimeout,
} from "./utils.js";
import { formatDateTime, t } from "./i18n.js";
import {
  formatConnectionAdvisory,
  formatConnectionVerdict,
  formatLatency,
  formatSpeed,
} from "./presentation.js";

const RESULTS_TIMEOUT_MS = 20000;
const RESULT_ID_REGEX = /^[0-9a-zA-Z]{8}$/;

const loadingView = document.getElementById("loadingView");
const resultView = document.getElementById("resultView");
const errorView = document.getElementById("errorView");
const errorMessage = document.querySelector("#errorView .error-message");
const errorCode = document.getElementById("errorCode");

function trimTrailingSlashes(value) {
  if (typeof value !== "string" || value.length === 0) return value;
  let end = value.length;
  while (end > 0 && value.codePointAt(end - 1) === 47) {
    end -= 1;
  }
  return value.slice(0, end);
}

function userError(key) {
  const err = new Error(key);
  err.userSafe = true;
  err.userKey = key;
  return err;
}

function resultErrorKey(statusCode) {
  if (statusCode === 404) return "error.resultNotFound";
  if (statusCode >= 500) return "error.resultServer";
  return "error.resultUnavailable";
}

function showError(key) {
  if (loadingView) loadingView.classList.add("hidden");
  if (resultView) resultView.classList.add("hidden");
  if (errorMessage) errorMessage.textContent = t(key);
  if (errorCode) {
    errorCode.textContent = "404";
    errorCode.classList.toggle("hidden", key !== "error.resultNotFound");
  }
  if (errorView) errorView.classList.remove("hidden");
}

function setText(el, text) {
  if (el) el.textContent = text;
}

function formatLatencyValue(v) {
  return formatLatency(v);
}

function bufferbloatBadgeClass(grade) {
  if (grade === "A+" || grade === "A") return "bb-good";
  if (grade === "B" || grade === "C") return "bb-mid";
  if (grade === "D" || grade === "F") return "bb-bad";
  return "";
}

function renderBufferbloatBadge(el, grade) {
  if (!el) return;
  el.classList.remove("bb-good", "bb-mid", "bb-bad");
  el.textContent = typeof grade === "string" && grade ? grade : "-";
  const badgeClass = bufferbloatBadgeClass(grade);
  if (badgeClass) el.classList.add(badgeClass);
}

function renderVerdict(d) {
  const el = document.getElementById("resultsVerdict");
  const advisoryEl = document.getElementById("resultsAdvisory");
  const values = {
    download: d.download_mbps,
    upload: d.upload_mbps,
    idleLatency: d.latency_ms,
    loadedLatency: d.loaded_latency_ms,
  };
  const verdict = formatConnectionVerdict(values);
  if (el) {
    el.textContent = verdict;
    el.classList.toggle("hidden", verdict === "");
  }
  const advisory = formatConnectionAdvisory(values);
  if (advisoryEl) {
    advisoryEl.textContent = advisory;
    advisoryEl.classList.toggle("hidden", advisory === "");
  }
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
      testedAtEl.textContent = formatDateTime(date, {
        dateStyle: "medium",
        timeStyle: "short",
      });
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
    throw userError(resultErrorKey(res.status));
  }
  return res.json();
}

function renderResult(d) {
  if (!d || typeof d !== "object") {
    showError("error.resultInvalidPayload");
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
    renderBufferbloatBadge(bufferbloatEl, d.bufferbloat_grade);
    renderVerdict(d);

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
    showError("error.resultRender");
  }
}

if (!loadingView || !resultView || !errorView) {
  console.error("Results page missing required view elements");
} else {
  const normalizedPath = trimTrailingSlashes(globalThis.location.pathname);
  const parts = normalizedPath.split("/").filter(Boolean);
  const id = parts.at(-1);
  if (!id || !RESULT_ID_REGEX.test(id)) {
    showError("error.resultInvalidId");
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
        showError("error.resultTimeout");
      } else if (err?.userSafe && err?.userKey) {
        showError(err.userKey);
      } else {
        showError("error.resultUnavailable");
      }
    }
  }
}
