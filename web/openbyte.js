/** Main entry: wiring, network detection, share/init. */

import { initElements } from "./state.js";
import {
  startTest,
  resetToIdle,
  handleShare,
} from "./speedtest-orchestrator.js";
import { checkServer, detectNetworkInfo, loadServerInfo } from "./network.js";
import { bindEvents } from "./events.js";

function init() {
  initElements();
  loadServerInfo();
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
