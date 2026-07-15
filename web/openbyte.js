/** Main entry: wiring, network detection, share/init. */

import { elements, initElements } from "./state.js";
import {
  startTest,
  resetToIdle,
  handleShare,
} from "./speedtest-orchestrator.js";
import { detectNetworkInfo, loadServerInfo } from "./network.js";

function bindEvents() {
  if (!elements.startBtn || !elements.restartBtn) {
    console.warn("Core UI elements missing; skipping event binding");
    return;
  }
  elements.startBtn.addEventListener("click", startTest);
  elements.restartBtn.addEventListener("click", resetToIdle);
  elements.cancelBtn?.addEventListener("click", resetToIdle);
  elements.shareBtn?.addEventListener("click", handleShare);
}

function init() {
  initElements();
  loadServerInfo();
  bindEvents();
  detectNetworkInfo();
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
