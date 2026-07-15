/** Server health, client IP discovery, and network display. */

import { getApiBase, elements, state, TEST_CONFIG } from "./state.js";
import { fetchWithTimeout, parseJSONOrThrow } from "./utils.js";

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

async function isHealthyServerCandidate(url) {
  try {
    const res = await fetchWithTimeout(
      url,
      {},
      TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
    );
    if (!res.ok) {
      await res.text().catch(() => {});
      return false;
    }

    let data;
    try {
      data = await res.json();
    } catch (err) {
      console.debug("failed to parse health response", err);
      await res.text().catch(() => {});
      return false;
    }

    return (
      data.status === "ok" || data.status === "healthy" || data.pong === true
    );
  } catch (err) {
    console.debug("server health candidate failed", err);
    return false;
  }
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

export function detectNetworkInfo() {
  const mainPing = fetch(`${getApiBase()}/ping`)
    .then((res) => parseJSONOrThrow(res))
    .then((data) => {
      if (data.client_ip) {
        if (data.ipv6) {
          state.networkInfo.ipv6 = data.client_ip;
        } else {
          state.networkInfo.ipv4 = data.client_ip;
        }
      }
    })
    .catch(() => {});

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

  const v4Ping = canProbe
    ? fetch(`${proto}//v4.${hostname}/api/v1/ping`, probeOpts)
        .then((res) => parseJSONOrThrow(res))
        .then((data) => {
          if (!data.ipv6 && data.client_ip) {
            state.networkInfo.ipv4 = data.client_ip;
          }
        })
        .catch(() => {})
    : Promise.resolve();

  const v6Ping = canProbe
    ? fetch(`${proto}//v6.${hostname}/api/v1/ping`, probeOpts)
        .then((res) => parseJSONOrThrow(res))
        .then((data) => {
          if (data.ipv6 && data.client_ip) {
            state.networkInfo.ipv6 = data.client_ip;
          }
        })
        .catch(() => {})
    : Promise.resolve();

  Promise.allSettled([mainPing, v4Ping, v6Ping]).then(updateNetworkDisplay);
}
