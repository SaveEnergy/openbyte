/** Manual theme override: system -> light -> dark, persisted per device. */

import { t } from "./i18n.js";

const STORAGE_KEY = "openbyte-theme";
const MODES = ["system", "light", "dark"];
const MODE_ICONS = { system: "◐", light: "☀", dark: "☾" };
const MODE_LABEL_KEYS = {
  system: "theme.systemNextLight",
  light: "theme.lightNextDark",
  dark: "theme.darkNextSystem",
};
let currentMode = storedMode();

function storedMode() {
  try {
    const value = localStorage.getItem(STORAGE_KEY);
    return value === "light" || value === "dark" ? value : "system";
  } catch {
    return "system";
  }
}

function persistMode(mode) {
  try {
    if (mode === "system") {
      localStorage.removeItem(STORAGE_KEY);
    } else {
      localStorage.setItem(STORAGE_KEY, mode);
    }
  } catch {
    // Storage unavailable (private browsing): theme still applies this page.
  }
}

function applyMode(mode) {
  const root = document.documentElement;
  if (mode === "light" || mode === "dark") {
    root.dataset.theme = mode;
  } else {
    delete root.dataset.theme;
  }
}

function updateToggle(button, mode) {
  const label = t(MODE_LABEL_KEYS[mode]);
  button.textContent = MODE_ICONS[mode];
  button.setAttribute("aria-label", label);
  button.title = label;
}

function wireToggle() {
  const button = document.getElementById("themeToggle");
  if (!button) return;
  updateToggle(button, currentMode);
  button.addEventListener("click", () => {
    const next = MODES[(MODES.indexOf(currentMode) + 1) % MODES.length];
    currentMode = next;
    persistMode(next);
    applyMode(next);
    updateToggle(button, next);
  });
}

applyMode(currentMode);
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", wireToggle);
} else {
  wireToggle();
}
