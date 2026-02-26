/** Warm-up phase detection and confidence-based early stop. */

import { TEST_CONFIG } from "./state.js";

const EARLY_STOP_WINDOW_MS = 500;
const EARLY_STOP_DELTA_THRESHOLD = 0.05;
const EARLY_STOP_STABLE_WINDOWS = 3;
const EARLY_STOP_MIN_WINDOWS = 2;

/**
 * Creates an early-stop detector: after warm-up, stops when rolling delta is below threshold
 * for N consecutive windows. Use with mutable endTime ref.
 * @param {() => boolean} settledFn - Returns true when warm-up has settled
 * @returns {{ record: (bytes: number, now: number) => boolean }}
 */
export function createEarlyStopDetector(settledFn) {
  let windowBytes = 0;
  let windowStart = 0;
  let detectorStart = 0;
  const recentSpeeds = [];

  return {
    record(bytes, now) {
      if (!settledFn()) return false;
      if (detectorStart === 0) {
        detectorStart = now;
        windowStart = now;
      }
      windowBytes += bytes;
      const elapsed = now - windowStart;
      if (elapsed < EARLY_STOP_WINDOW_MS) return false;
      const speed = (windowBytes * 8) / (elapsed / 1000);
      recentSpeeds.push(speed);
      windowBytes = 0;
      windowStart = now;
      if (recentSpeeds.length < EARLY_STOP_MIN_WINDOWS) return false;
      const recent = recentSpeeds.slice(-EARLY_STOP_STABLE_WINDOWS);
      if (recent.length < EARLY_STOP_STABLE_WINDOWS) return false;
      const avg = recent.reduce((a, b) => a + b, 0) / recent.length;
      if (avg <= 0) return false;
      const maxDelta = Math.max(...recent.map((s) => Math.abs(s - avg) / avg));
      return maxDelta < EARLY_STOP_DELTA_THRESHOLD;
    },
  };
}

export function createWarmUpDetector(durationMs) {
  const windowMs = TEST_CONFIG.WARMUP_WINDOW_MS;
  const stabilityThreshold = TEST_CONFIG.WARMUP_STABILITY_THRESHOLD;
  const requiredStableWindows = TEST_CONFIG.WARMUP_REQUIRED_WINDOWS;
  const maxGraceMs = Math.min(
    durationMs * TEST_CONFIG.WARMUP_MAX_GRACE_RATIO,
    TEST_CONFIG.WARMUP_MAX_GRACE_MS,
  );

  let windowBytes = 0;
  let windowStart = 0;
  let detectorStart = 0;
  let recentSpeeds = [];
  let settled = false;

  return {
    settled() {
      return settled;
    },
    record(bytes, now) {
      if (settled) return;
      if (detectorStart === 0) {
        detectorStart = now;
        windowStart = now;
      }
      windowBytes += bytes;
      const windowElapsed = now - windowStart;

      if (windowElapsed >= windowMs) {
        const speed = (windowBytes * 8) / (windowElapsed / 1000);
        recentSpeeds.push(speed);
        windowBytes = 0;
        windowStart = now;

        if (recentSpeeds.length >= requiredStableWindows) {
          const recent = recentSpeeds.slice(-requiredStableWindows);
          const avg = recent.reduce((a, b) => a + b, 0) / recent.length;
          if (avg === 0) {
            settled = true;
            return;
          }
          const maxDev = Math.max(
            ...recent.map((s) => Math.abs(s - avg) / avg),
          );
          if (maxDev < stabilityThreshold) settled = true;
        }

        if (now - detectorStart > maxGraceMs) settled = true;
      }
    },
  };
}
