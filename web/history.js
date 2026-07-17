/** Optional recent-results history stored locally on this device. */

import { formatDateTime, formatRelativeTime, t } from "./i18n.js";
import { formatLatency, formatSpeed } from "./presentation.js";
import { isHistoryEnabled } from "./preferences.js";

const STORAGE_KEY = "openbyte-history";
const MAX_STORED_ENTRIES = 10;
const MAX_DISPLAY_ENTRIES = 5;

function isValidEntry(entry) {
  return (
    entry !== null &&
    typeof entry === "object" &&
    Number.isFinite(entry.ts) &&
    Number.isFinite(entry.down) &&
    Number.isFinite(entry.up)
  );
}

export function loadHistory() {
  if (!isHistoryEnabled()) return [];
  try {
    const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || "[]");
    return Array.isArray(parsed) ? parsed.filter(isValidEntry) : [];
  } catch {
    return [];
  }
}

export function saveHistoryEntry(entry) {
  if (!isHistoryEnabled() || !isValidEntry(entry)) return;
  try {
    const entries = [entry, ...loadHistory()].slice(0, MAX_STORED_ENTRIES);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
  } catch {
    // Storage unavailable: history is a convenience, ignore.
  }
}

function formatWhen(ts) {
  const minutes = Math.round((Date.now() - ts) / 60000);
  if (minutes < 1) return t("history.justNow");
  if (minutes < 60) {
    return formatRelativeTime(-minutes, "minute", { numeric: "always" });
  }
  const hours = Math.round(minutes / 60);
  if (hours < 24) {
    return formatRelativeTime(-hours, "hour", { numeric: "always" });
  }
  return formatDateTime(new Date(ts), { dateStyle: "medium" });
}

function speedText(mbps) {
  const formatted = formatSpeed(mbps);
  return `${formatted.value} ${formatted.unit}`;
}

export function renderHistory(listEl, sectionEl) {
  if (!listEl) return;
  const entries = loadHistory();
  listEl.replaceChildren();
  if (sectionEl) {
    sectionEl.classList.toggle("hidden", entries.length === 0);
  }

  for (const entry of entries.slice(0, MAX_DISPLAY_ENTRIES)) {
    const item = document.createElement("li");
    item.className = "history-item";

    const when = document.createElement("span");
    when.className = "history-when";
    when.textContent = formatWhen(entry.ts);

    const speeds = document.createElement("span");
    speeds.className = "history-speeds";
    speeds.textContent = `↓ ${speedText(entry.down)}  ↑ ${speedText(entry.up)}`;

    const meta = document.createElement("span");
    meta.className = "history-meta";
    const formattedLatency = formatLatency(entry.latency);
    const latency = formattedLatency === "-" ? "—" : formattedLatency;
    meta.textContent = entry.grade ? `${latency} · ${entry.grade}` : latency;

    item.append(when, speeds, meta);
    listEl.append(item);
  }
}
