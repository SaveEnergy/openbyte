/** Explicit system/light/dark theme preference, persisted per device. */

const STORAGE_KEY = "openbyte-theme";
const MODES = new Set(["system", "light", "dark"]);
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

function wireThemeOptions() {
  const controls = [...document.querySelectorAll('input[name="themeMode"]')];
  if (controls.length === 0) return;

  for (const control of controls) {
    control.checked = control.value === currentMode;
    control.addEventListener("change", () => {
      if (!control.checked || !MODES.has(control.value)) return;
      currentMode = control.value;
      persistMode(currentMode);
      applyMode(currentMode);
    });
  }
}

applyMode(currentMode);
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", wireThemeOptions, { once: true });
} else {
  wireThemeOptions();
}
