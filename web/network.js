/** Server health and network info (barrel). */

import { getApiBase, elements, state } from "./state.js";
import { isHealthyServerCandidate } from "./network-health.js";

export { detectNetworkInfo, updateNetworkDisplay } from "./network-probes.js";

const fallbackServerName = "openByte Server";

export function resolveServerName() {
  return normalizeServerName(state.serverName);
}

export async function loadServerInfo() {
  try {
    const response = await fetch(`${getApiBase()}/version`, {
      cache: "no-store",
    });
    if (!response.ok) {
      await response.text().catch(() => {});
      throw new Error(`version endpoint returned ${response.status}`);
    }
    const data = await response.json();
    setServerName(data?.server_name);
  } catch (e) {
    console.debug("Server info load failed:", e);
    setServerName(state.serverName);
  }
}

function setServerName(name) {
  state.serverName = normalizeServerName(name);
  if (elements.serverName) {
    elements.serverName.textContent = state.serverName;
  }
}

function normalizeServerName(name) {
  const value = typeof name === "string" ? name.trim() : "";
  return value || fallbackServerName;
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
