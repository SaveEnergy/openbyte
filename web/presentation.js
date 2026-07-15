/** Main-thread localized formatting for speed-test values and verdicts. */

import { formatNumber, t } from "./i18n.js";
import { computeConnectionVerdict } from "./utils.js";

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

export function formatConnectionVerdict(values) {
  const verdict = computeConnectionVerdict(values);
  if (!verdict) return "";
  const parts = [t(verdict.key)];
  if (verdict.warningKey) parts.push(t(verdict.warningKey));
  return parts.join(" ");
}
