/** Manual theme override: system -> light -> dark, persisted per device. */

const STORAGE_KEY = "openbyte-theme";
const MODES = ["system", "light", "dark"];
const MODE_ICONS = { system: "◐", light: "☀", dark: "☾" };
const MODE_LABELS = {
  system: "Theme follows system preference. Activate for light theme.",
  light: "Light theme active. Activate for dark theme.",
  dark: "Dark theme active. Activate to follow system preference.",
};

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
  button.textContent = MODE_ICONS[mode];
  button.setAttribute("aria-label", MODE_LABELS[mode]);
  button.title = MODE_LABELS[mode];
}

function wireToggle() {
  const button = document.getElementById("themeToggle");
  if (!button) return;
  updateToggle(button, storedMode());
  button.addEventListener("click", () => {
    const next = MODES[(MODES.indexOf(storedMode()) + 1) % MODES.length];
    persistMode(next);
    applyMode(next);
    updateToggle(button, next);
  });
}

applyMode(storedMode());
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", wireToggle);
} else {
  wireToggle();
}
