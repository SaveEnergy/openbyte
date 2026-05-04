/** Main page event binding. */

import { elements } from "./state.js";

export function bindEvents(extraHandlers) {
  if (!elements.startBtn || !elements.restartBtn) {
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
}
