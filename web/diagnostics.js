/** Internal diagnostics: peak/sustained/volatility for tuning, not default UI. */

/**
 * Creates a diagnostics collector for throughput tests.
 * Tracks window-based speeds during measurement phase; computes peak, sustained, volatility.
 * @param {number} windowMs - Rolling window duration in ms
 * @returns {{ record: (bytes: number, now: number, isMeasuring: boolean) => void, finish: (stopReason: string) => object }}
 */
export function createDiagnosticsCollector(windowMs = 500) {
  let windowBytes = 0;
  let windowStart = 0;
  const windowSpeeds = [];

  return {
    record(bytes, now, isMeasuring) {
      if (!isMeasuring) return;
      if (windowStart === 0) windowStart = now;
      windowBytes += bytes;
      const elapsed = now - windowStart;
      if (elapsed >= windowMs && elapsed > 0) {
        const speedMbps = (windowBytes * 8) / (elapsed / 1000) / 1_000_000;
        windowSpeeds.push(speedMbps);
        windowBytes = 0;
        windowStart = now;
      }
    },
    finish(stopReason = "duration") {
      if (windowSpeeds.length === 0) {
        return { peakMbps: 0, sustainedMbps: 0, volatility: 0, stopReason };
      }
      const peak = Math.max(...windowSpeeds);
      const sustained =
        windowSpeeds.reduce((a, b) => a + b, 0) / windowSpeeds.length;
      const variance =
        windowSpeeds.reduce((acc, s) => acc + (s - sustained) ** 2, 0) /
        windowSpeeds.length;
      const volatility = Math.sqrt(variance);
      return {
        peakMbps: Math.round(peak * 100) / 100,
        sustainedMbps: Math.round(sustained * 100) / 100,
        volatility: Math.round(volatility * 100) / 100,
        stopReason,
      };
    },
  };
}
