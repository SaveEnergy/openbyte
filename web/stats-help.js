/** Shared metric explanations for the "What do these numbers mean?" panel. */

import { t } from "./i18n.js";

const METRIC_EXPLANATIONS = [
  ["metric.idleLatency", "metric.idleLatencyDescription"],
  ["metric.jitter", "metric.jitterDescription"],
  ["metric.loadedLatency", "metric.loadedLatencyDescription"],
  ["metric.bufferbloat", "metric.bufferbloatDescription"],
];

function renderMetricExplanations() {
  const list = document.getElementById("statsHelpList");
  if (!list) return;
  list.replaceChildren();
  for (const [termKey, explanationKey] of METRIC_EXPLANATIONS) {
    const dt = document.createElement("dt");
    dt.textContent = t(termKey);
    const dd = document.createElement("dd");
    dd.textContent = t(explanationKey);
    list.append(dt, dd);
  }
}

renderMetricExplanations();
