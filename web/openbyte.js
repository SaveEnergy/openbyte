/** Main entry: wiring, network detection, share/init. */

import { elements, initElements, state, TEST_CONFIG } from "./state.js";
import {
  startTest,
  resetToIdle,
  handleCancel,
  handleShare,
} from "./speedtest-orchestrator.js";
import { checkServer, detectNetworkInfo, loadServerInfo } from "./network.js";

function bindEvents() {
  if (!elements.startBtn || !elements.restartBtn) {
    console.warn("Core UI elements missing; skipping event binding");
    return;
  }
  elements.startBtn.addEventListener("click", startTest);
  elements.restartBtn.addEventListener("click", resetToIdle);
  elements.cancelBtn?.addEventListener("click", handleCancel);
  elements.shareBtn?.addEventListener("click", handleShare);
}

function init() {
  initElements();
  loadServerInfo();
  bindEvents();
  detectNetworkInfo();
  setInterval(() => {
    if (!state.isRunning && state.phase === "idle") checkServer();
  }, TEST_CONFIG.SERVER_RECHECK_MS);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
