/** Main-thread localized formatting for speed-test values and advisories. */

import { formatNumber, t } from "./i18n.js";
import { computeBufferbloatGrade } from "./utils.js";

export function formatSpeed(speed) {
  const safeSpeed =
    typeof speed === "number" && Number.isFinite(speed) && speed >= 0 ? speed : 0;
  if (safeSpeed >= 1000) {
    return {
      value: formatNumber(safeSpeed / 1000, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }),
      unit: "Gbps",
    };
  }
  return {
    value: formatNumber(safeSpeed, {
      minimumFractionDigits: 1,
      maximumFractionDigits: 1,
    }),
    unit: "Mbps",
  };
}

export function formatSpeedText(speed) {
  const formatted = formatSpeed(speed);
  return `${formatted.value} ${formatted.unit}`;
}

export function formatLatency(value) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return "-";
  }
  return `${formatNumber(value, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })} ms`;
}

export function formatLoadedLatencyAdvisory({ idleLatency, loadedLatency }) {
  const grade = computeBufferbloatGrade(idleLatency, loadedLatency);
  return grade === "C" || grade === "D" || grade === "F"
    ? t("result.loadedLatencyAdvisory")
    : "";
}
