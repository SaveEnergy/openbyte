/** Settings modal and localStorage persistence. */

import { state, elements, modal } from "./state.js";
import { notifySettingsSaved } from "./ui.js";

function initSettingsModal() {
  if (
    !elements.settingsModal ||
    !elements.showSettings ||
    !elements.closeSettings
  )
    return;

  const focusFirstSetting = () => {
    if (elements.duration) elements.duration.focus();
  };
  let previousBodyOverflow = "";

  const lockBodyScroll = () => {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  };

  const unlockBodyScroll = () => {
    document.body.style.overflow = previousBodyOverflow;
  };

  const openModal = () => {
    modal.lastTrigger = document.activeElement;
    elements.settingsModal.showModal();
    lockBodyScroll();
    requestAnimationFrame(focusFirstSetting);
  };

  const closeModal = () => {
    elements.settingsModal.close();
    if (modal.lastTrigger && typeof modal.lastTrigger.focus === "function") {
      modal.lastTrigger.focus();
    }
  };

  elements.showSettings.addEventListener("click", openModal);
  elements.closeSettings.addEventListener("click", closeModal);
  elements.settingsModal.addEventListener("cancel", (e) => {
    e.preventDefault();
    closeModal();
  });
  elements.settingsModal.addEventListener("click", (e) => {
    if (e.target === elements.settingsModal) closeModal();
  });
  elements.settingsModal.addEventListener("close", unlockBodyScroll);
}

export function loadSettings() {
  const saved = localStorage.getItem("obyte-settings");
  if (saved) {
    try {
      const s = JSON.parse(saved);
      if (
        typeof s.duration === "number" &&
        Number.isFinite(s.duration) &&
        s.duration > 0
      ) {
        state.settings.duration = s.duration;
      }
      if (
        typeof s.streams === "number" &&
        Number.isFinite(s.streams) &&
        s.streams > 0
      ) {
        state.settings.streams = s.streams;
      }
      if (elements.duration) elements.duration.value = state.settings.duration;
      if (elements.streams) elements.streams.value = state.settings.streams;
    } catch (e) {
      console.warn("Failed to parse saved settings:", e);
    }
  }
}

export function saveSettings() {
  if (!elements.duration || !elements.streams) return;
  const d = Number.parseInt(elements.duration.value, 10);
  const s = Number.parseInt(elements.streams.value, 10);
  if (Number.isFinite(d) && d > 0) state.settings.duration = d;
  if (Number.isFinite(s) && s > 0) state.settings.streams = s;
  localStorage.setItem("obyte-settings", JSON.stringify(state.settings));
  notifySettingsSaved();
}

export function bindEvents(extraHandlers) {
  if (
    !elements.startBtn ||
    !elements.restartBtn ||
    !elements.duration ||
    !elements.streams
  ) {
    console.warn("Core UI elements missing; skipping event binding");
    return;
  }
  elements.startBtn.addEventListener("click", extraHandlers.startTest);
  elements.restartBtn.addEventListener("click", extraHandlers.resetToIdle);
  if (elements.cancelBtn) {
    elements.cancelBtn.addEventListener("click", extraHandlers.resetToIdle);
  }
  if (elements.shareBtn) {
    elements.shareBtn.addEventListener("click", extraHandlers.handleShare);
  }
  initSettingsModal();
  elements.duration.addEventListener("change", saveSettings);
  elements.streams.addEventListener("change", saveSettings);

  if (elements.serverSelect) {
    elements.serverSelect.addEventListener(
      "change",
      extraHandlers.onServerChange,
    );
  }
}
