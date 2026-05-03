/** Main entry: wiring, settings, share/init. */

import { initElements } from "./state.js";
import {
  startTest,
  resetToIdle,
  handleShare,
} from "./speedtest-orchestrator.js";
import { checkServer, detectNetworkInfo } from "./network.js";
import { bindEvents, loadSettings } from "./settings.js";

function init() {
  initElements();
  loadSettings();
  checkServer();
  bindEvents({
    startTest,
    resetToIdle,
    handleShare,
  });
  detectNetworkInfo();
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
