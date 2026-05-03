/** Server health and network info (barrel). */

import { getApiBase, elements } from "./state.js";
import { detectNetworkInfo, updateNetworkDisplay } from "./network-probes.js";
import { isHealthyServerCandidate } from "./network-health.js";

export { detectNetworkInfo, updateNetworkDisplay };

export function resolveServerName() {
  return "Current Server";
}

function setServerOnlineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("error", "warning");
    elements.serverDot.classList.add("connected");
  }
  if (elements.serverText) elements.serverText.textContent = "Ready";
}

function setServerOfflineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("connected", "warning");
    elements.serverDot.classList.add("error");
  }
  if (elements.serverText) elements.serverText.textContent = "Offline";
}

export async function checkServer() {
  const candidates = ["/health", `${getApiBase()}/ping`];

  try {
    for (const url of candidates) {
      if (await isHealthyServerCandidate(url)) {
        setServerOnlineUI();
        return;
      }
    }
    throw new Error("Server offline");
  } catch (e) {
    console.debug("Server health check failed:", e);
    setServerOfflineUI();
  }
}
