/** Main entry: wiring, settings, share/init. */

import { initElements } from "./state.js";
import {
  startTest,
  resetToIdle,
  handleShare,
} from "./speedtest-orchestrator.js";
import { loadServers, detectNetworkInfo, onServerChange } from "./network.js";
import { bindEvents, loadSettings } from "./settings.js";
import { showError } from "./ui.js";

function init() {
  initElements();
  loadSettings();
  loadServers().catch((err) => {
    showError(err?.message || "Failed to load servers");
  });
  bindEvents({
    startTest,
    resetToIdle,
    handleShare,
    onServerChange,
  });
  detectNetworkInfo();
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
