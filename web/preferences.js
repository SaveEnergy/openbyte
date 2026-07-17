/** Shared disclosure behavior and device-storage preferences. */

import "./i18n.js";
import "./theme.js";

const HISTORY_KEY = "openbyte-history";
const HISTORY_ENABLED_KEY = "openbyte-history-enabled";
export const HISTORY_PREFERENCE_EVENT = "openbyte:history-preference";

export function isHistoryEnabled() {
  try {
    return localStorage.getItem(HISTORY_ENABLED_KEY) === "true";
  } catch {
    return false;
  }
}

function clearStoredHistory() {
  try {
    localStorage.removeItem(HISTORY_KEY);
  } catch {
    // Storage unavailable: there is nothing useful to clear.
  }
}

function setHistoryEnabled(enabled) {
  try {
    if (enabled) {
      localStorage.setItem(HISTORY_ENABLED_KEY, "true");
    } else {
      localStorage.removeItem(HISTORY_ENABLED_KEY);
      localStorage.removeItem(HISTORY_KEY);
    }
    return true;
  } catch {
    return false;
  }
}

function wireDisclosure(menu) {
  const trigger = menu.querySelector("summary");

  menu.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || !menu.open) return;
    event.preventDefault();
    menu.open = false;
    trigger?.focus();
  });

  menu.addEventListener("focusout", () => {
    setTimeout(() => {
      if (!menu.contains(document.activeElement)) menu.open = false;
    });
  });

  document.addEventListener("pointerdown", (event) => {
    if (menu.open && !menu.contains(event.target)) menu.open = false;
  });
}

function wireHistoryPreference(control) {
  control.checked = isHistoryEnabled();
  if (!control.checked) clearStoredHistory();

  control.addEventListener("change", () => {
    if (!setHistoryEnabled(control.checked)) control.checked = false;
    document.dispatchEvent(new Event(HISTORY_PREFERENCE_EVENT));
  });
}

function init() {
  const menu = document.getElementById("preferencesMenu");
  if (menu) wireDisclosure(menu);

  const historyControl = document.getElementById("historyPreference");
  if (historyControl) wireHistoryPreference(historyControl);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}
