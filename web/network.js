/** Server health, client IP discovery, and network display. */

import { getApiBase, elements, state, TEST_CONFIG } from "./state.js";
import { fetchWithTimeout, parseJSONOrThrow } from "./utils.js";

const fallbackServerName = "openByte Server";

function isIdle() {
  return !state.isRunning && state.phase === "idle";
}

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

function setStartAvailability(online) {
  state.serverOnline = online;
  if (elements.startBtn) {
    elements.startBtn.disabled = !online;
  }
  if (elements.startBtnHint) {
    elements.startBtnHint.textContent = online
      ? "Click to test your speed"
      : "Server offline — retrying";
  }
}

function setServerOnlineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("error");
    elements.serverDot.classList.add("connected");
  }
  if (elements.serverText) elements.serverText.textContent = "Ready";
  setStartAvailability(true);
}

function setServerOfflineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("connected");
    elements.serverDot.classList.add("error");
  }
  if (elements.serverText) elements.serverText.textContent = "Offline";
  setStartAvailability(false);
}

function startsWithDigit(value) {
  if (!value || typeof value !== "string") return false;
  const code = value.codePointAt(0);
  return typeof code === "number" && code >= 48 && code <= 57;
}

export function updateNetworkDisplay() {
  const pending = state.networkInfo.complete ? "Not detected" : "Detecting…";
  const ipv4 = state.networkInfo.ipv4 || pending;
  const ipv6 = state.networkInfo.ipv6 || pending;
  for (const element of [elements.idleNetworkIPv4, elements.networkIPv4]) {
    if (element) element.textContent = ipv4;
  }
  for (const element of [elements.idleNetworkIPv6, elements.networkIPv6]) {
    if (element) element.textContent = ipv6;
  }
  if (elements.idleNetworkInfo) {
    elements.idleNetworkInfo.setAttribute(
      "aria-busy",
      state.networkInfo.complete ? "false" : "true",
    );
  }
}

async function discoverAddress(url, options, shouldUpdate = () => true) {
  try {
    const response = await fetchWithTimeout(
      url,
      options,
      TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
    );
    const data = await parseJSONOrThrow(response);
    if (data.client_ip && shouldUpdate()) {
      state.networkInfo[data.ipv6 ? "ipv6" : "ipv4"] = data.client_ip;
      updateNetworkDisplay();
    }
    return true;
  } catch (err) {
    console.debug("IP discovery failed", err);
    return false;
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

/** Periodic idle re-check: probe the same-origin ping and update readiness. */
export async function checkServer() {
  const online = await discoverAddress(
    `${getApiBase()}/ping`,
    { cache: "no-store" },
    isIdle,
  );
  if (online) setServerOnlineUI();
  else setServerOfflineUI();
}

export function detectNetworkInfo() {
  const discoveryGeneration = state.runGeneration;
  const canUpdateStartupAddress = () =>
    state.phase !== "results" &&
    (state.runGeneration === discoveryGeneration ||
      (state.runGeneration === discoveryGeneration + 1 && state.isRunning));
  const sameOriginProbe = discoverAddress(
    `${getApiBase()}/ping`,
    { cache: "no-store" },
    canUpdateStartupAddress,
  );
  void sameOriginProbe.then((ready) => {
    if (ready) setServerOnlineUI();
    else setServerOfflineUI();
  });
  const probes = [sameOriginProbe];

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
      discoverAddress(
        `${proto}//v4.${hostname}/api/v1/ping`,
        probeOpts,
        canUpdateStartupAddress,
      ),
      discoverAddress(
        `${proto}//v6.${hostname}/api/v1/ping`,
        probeOpts,
        canUpdateStartupAddress,
      ),
    );
  }

  return Promise.allSettled(probes).then((results) => {
    state.networkInfo.complete = true;
    updateNetworkDisplay();
    return results;
  });
}
