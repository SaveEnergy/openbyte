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
 * Partial runs (no upload figure) grade on download and latency only.
 */
export function computeConnectionVerdict({
  download,
  upload,
  idleLatency,
  loadedLatency,
  partial,
}) {
  if (!Number.isFinite(download) || download <= 0) return "";

  let verdict;
  if (partial === true) {
    if (download >= 500) {
      verdict =
        "Exceptional download speed — ample for multiple 4K streams and large downloads.";
    } else if (download >= 100) {
      verdict =
        "Excellent download speed — smooth 4K streaming and fast downloads.";
    } else if (download >= 25) {
      verdict = "Good download speed — comfortable HD streaming and browsing.";
    } else if (download >= 10) {
      verdict = "Modest download speed — fine for browsing and music.";
    } else {
      verdict = "Slow download speed — expect buffering and long downloads.";
    }
  } else if (download >= 500 && upload >= 100) {
    verdict =
      "Exceptional connection — handles 4K streaming, cloud backups, and busy households with ease.";
  } else if (download >= 100 && upload >= 20) {
    verdict =
      "Excellent connection — smooth 4K streaming, video calls, and gaming.";
  } else if (download >= 25 && upload >= 5) {
    verdict =
      "Good connection — comfortable HD streaming and stable video calls.";
  } else if (download >= 10) {
    verdict =
      "Modest connection — fine for browsing and music; large downloads take a while.";
  } else {
    verdict = "Slow connection — expect buffering and long download times.";
  }

  const grade = computeBufferbloatGrade(idleLatency, loadedLatency);
  if (grade === "C" || grade === "D" || grade === "F") {
    verdict +=
      " Latency rises noticeably under load (bufferbloat), which can cause lag in calls and games while the connection is busy.";
  }
  return verdict;
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export function formatSpeed(speed) {
  if (typeof speed !== "number" || !Number.isFinite(speed) || speed < 0)
    speed = 0;
  if (speed >= 1000) return { value: (speed / 1000).toFixed(2), unit: "Gbps" };
  return { value: speed.toFixed(1), unit: "Mbps" };
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
