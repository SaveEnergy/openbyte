/** Warm-up phase detection for throughput measurement. */

import { TEST_CONFIG } from "./state.js";

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
