/** HTTP/network helpers and shared utilities. */

const RETRY_AFTER_DEFAULT_MS = 1000;
const RETRY_AFTER_MAX_MS = 120000;

export function computeBufferbloatGrade(idleLatency, loadedLatency) {
  if (!Number.isFinite(idleLatency) || !Number.isFinite(loadedLatency))
    return null;
  if (idleLatency <= 0 || loadedLatency <= 0) return null;
  const increase = loadedLatency - idleLatency;
  if (increase < 5) return "A+";
  if (increase < 15) return "A";
  if (increase < 30) return "B";
  if (increase < 60) return "C";
  if (increase < 150) return "D";
  return "F";
}

/**
 * One-sentence plain-language interpretation of the measured results.
 * Partial runs interpret the measured download without assigning a generic
 * under-load grade from download-only latency.
 */
export function computeConnectionVerdict({
  download,
  upload,
  idleLatency,
  loadedLatency,
  partial,
}) {
  if (!Number.isFinite(download) || download <= 0) return null;

  let key;
  if (partial === true) {
    if (download >= 500) {
      key = "verdict.partial.exceptional";
    } else if (download >= 100) {
      key = "verdict.partial.excellent";
    } else if (download >= 25) {
      key = "verdict.partial.good";
    } else if (download >= 10) {
      key = "verdict.partial.modest";
    } else {
      key = "verdict.partial.slow";
    }
  } else if (download >= 500 && upload >= 100) {
    key = "verdict.complete.exceptional";
  } else if (download >= 100 && upload >= 20) {
    key = "verdict.complete.excellent";
  } else if (download >= 25 && upload >= 5) {
    key = "verdict.complete.good";
  } else if (download >= 10) {
    key = "verdict.complete.modest";
  } else {
    key = "verdict.complete.slow";
  }

  const grade =
    partial === true ? null : computeBufferbloatGrade(idleLatency, loadedLatency);
  const warningKey =
    grade === "C" || grade === "D" || grade === "F"
      ? "verdict.bufferbloatWarning"
      : null;
  return { key, warningKey };
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export function createCodedError(code, message) {
  const error = new Error(message);
  error.code = code;
  return error;
}

export async function consumeErrorBody(res) {
  try {
    await res.text();
  } catch (err) {
    console.debug("failed to read error response body", err);
  }
}

export function retryAfterMs(response, fallbackMs = RETRY_AFTER_DEFAULT_MS) {
  if (!response?.headers) return fallbackMs;
  const value = response.headers.get("Retry-After");
  if (!value) return fallbackMs;
  const seconds = Number.parseInt(value, 10);
  if (!Number.isFinite(seconds) || seconds < 1) return fallbackMs;
  return Math.min(seconds * 1000, RETRY_AFTER_MAX_MS);
}

export function isNetworkError(err) {
  if (!err) return false;
  if (err.name === "AbortError") return false;
  if (err.name === "TypeError") return true;
  const message = (err.message || "").toLowerCase();
  return (
    message.includes("network") ||
    message.includes("failed to fetch") ||
    message.includes("http2")
  );
}

export const parseJSONOrThrow = (res) =>
  res.ok
    ? res.json()
    : res.text().then(() => {
        throw new Error(`HTTP ${res.status}`);
      });

export function fetchWithTimeout(url, options, timeoutMs) {
  if (typeof AbortController === "undefined" || !timeoutMs) {
    return fetch(url, options);
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  const externalSignal = options?.signal;
  let onAbort = null;
  if (externalSignal) {
    if (externalSignal.aborted) {
      controller.abort();
    } else {
      onAbort = () => controller.abort();
      externalSignal.addEventListener("abort", onAbort, { once: true });
    }
  }

  const opts = { ...options, signal: controller.signal };
  return fetch(url, opts).finally(() => {
    clearTimeout(timer);
    if (onAbort && externalSignal) {
      externalSignal.removeEventListener("abort", onAbort);
    }
  });
}
