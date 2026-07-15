/** Server health, client IP discovery, and network display. */

import { getApiBase, elements, state, TEST_CONFIG } from "./state.js";
import { fetchWithTimeout, parseJSONOrThrow } from "./utils.js";

const fallbackServerName = "openByte Server";

export function resolveServerName() {
  return normalizeServerName(state.serverName);
}

export async function loadServerInfo() {
  try {
    const response = await fetchWithTimeout(
      `${getApiBase()}/version`,
      { cache: "no-store" },
      TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
    );
    if (!response.ok) {
      await response.text().catch(() => {});
      throw new Error(`version endpoint returned ${response.status}`);
    }
    const data = await response.json();
    setServerName(data?.server_name);
    setServerOnlineUI();
  } catch (e) {
    console.debug("Server info load failed:", e);
    setServerName(state.serverName);
    setServerOfflineUI();
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
    elements.serverDot.classList.remove("error");
    elements.serverDot.classList.add("connected");
  }
  if (elements.serverText) elements.serverText.textContent = "Ready";
}

function setServerOfflineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("connected");
    elements.serverDot.classList.add("error");
  }
  if (elements.serverText) elements.serverText.textContent = "Offline";
}

function startsWithDigit(value) {
  if (!value || typeof value !== "string") return false;
  const code = value.codePointAt(0);
  return typeof code === "number" && code >= 48 && code <= 57;
}

export function updateNetworkDisplay() {
  if (elements.networkIPv4) {
    elements.networkIPv4.textContent = state.networkInfo.ipv4 || "-";
  }
  if (elements.networkIPv6) {
    elements.networkIPv6.textContent = state.networkInfo.ipv6 || "-";
  }
}

async function discoverAddress(url, options) {
  try {
    const response = await fetchWithTimeout(
      url,
      options,
      TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
    );
    const data = await parseJSONOrThrow(response);
    if (!data.client_ip) return;
    state.networkInfo[data.ipv6 ? "ipv6" : "ipv4"] = data.client_ip;
    updateNetworkDisplay();
  } catch (err) {
    console.debug("IP discovery failed", err);
  }
}

export function getNextHopProtocol() {
  try {
    const entries = performance.getEntriesByType("resource");
    for (let i = entries.length - 1; i >= 0; i--) {
      const entry = entries[i];
      if (
        String(entry.name || "").includes(`${getApiBase()}/ping`) &&
        entry.nextHopProtocol
      ) {
        return entry.nextHopProtocol;
      }
    }
  } catch (err) {
    console.debug("protocol detection failed", err);
  }
  return "";
}

export function detectNetworkInfo() {
  const probes = [
    discoverAddress(`${getApiBase()}/ping`, { cache: "no-store" }),
  ];

  const hostname = globalThis.location.hostname;
  const canProbe =
    hostname &&
    hostname !== "localhost" &&
    !hostname.startsWith("v4.") &&
    !hostname.startsWith("v6.") &&
    !hostname.startsWith("[") &&
    !startsWithDigit(hostname);

  const probeOpts = { cache: "no-store", credentials: "omit", mode: "cors" };
  const proto = globalThis.location.protocol;

  if (canProbe) {
    probes.push(
      discoverAddress(`${proto}//v4.${hostname}/api/v1/ping`, probeOpts),
      discoverAddress(`${proto}//v6.${hostname}/api/v1/ping`, probeOpts),
    );
  }

  return Promise.allSettled(probes);
}
