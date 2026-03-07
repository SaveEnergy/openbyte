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

function normalizeMessage(value) {
  return typeof value === "string" ? value.trim() : "";
}

function responseMessageFromJSON(payload) {
  if (!payload || typeof payload !== "object") return "";
  return (
    normalizeMessage(payload.error) ||
    normalizeMessage(payload.message) ||
    normalizeMessage(payload.detail)
  );
}

export async function readErrorResponseMessage(res, fallbackMessage) {
  if (!res) return normalizeMessage(fallbackMessage);

  const fallback = normalizeMessage(fallbackMessage) || `HTTP ${res.status}`;
  const contentType = res.headers.get("Content-Type") || "";
  const isJSON = contentType.includes("application/json");
  const responseClone = typeof res.clone === "function" ? res.clone() : null;
  let parseFailed = false;

  try {
    if (isJSON) {
      const payload = await res.json();
      const message = responseMessageFromJSON(payload);
      return message || fallback;
    } else {
      const text = normalizeMessage(await res.text());
      if (text) return text;
      return fallback;
    }
  } catch (err) {
    parseFailed = true;
    console.debug("failed to parse error response body", err);
  }

  if (responseClone && parseFailed) {
    try {
      const text = normalizeMessage(await responseClone.text());
      if (text) return text;
    } catch (err) {
      console.debug("failed to read fallback error response body", err);
    }
  }

  return fallback;
}

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
