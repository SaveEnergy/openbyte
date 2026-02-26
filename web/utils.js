/** HTTP/network helpers and shared utilities. */

import { TEST_CONFIG } from "./state.js";

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

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export function retryAfterMs(
  response,
  fallbackMs = TEST_CONFIG.RETRY_AFTER_DEFAULT_MS,
) {
  if (!response?.headers) return fallbackMs;
  const value = response.headers.get("Retry-After");
  if (!value) return fallbackMs;
  const seconds = Number.parseInt(value, 10);
  if (!Number.isFinite(seconds) || seconds < 1) return fallbackMs;
  return Math.min(seconds * 1000, TEST_CONFIG.RETRY_AFTER_MAX_MS);
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

export function isSameOriginURL(url) {
  try {
    const parsed = new URL(url, globalThis.location.origin);
    return parsed.origin === globalThis.location.origin;
  } catch (e) {
    console.debug("invalid URL for same-origin check", e);
    return false;
  }
}

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
