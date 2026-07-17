/** Server health, client IP discovery, and network display. */

import { getApiBase, elements, state, TEST_CONFIG } from "./state.js";
import { t } from "./i18n.js";
import { fetchWithTimeout, parseJSONOrThrow } from "./utils.js";

const fallbackServerName = "openByte Server";

/**
 * Negative cache for the optional v4./v6. probe hosts. Deployments without
 * those DNS records fail the probe on every load and the browser logs a
 * network error each time; remembering the failure for this tab keeps the
 * console clean until the TTL expires and the probe becomes eager again.
 */
const PROBE_SKIP_KEY = "openbyte-probe-skip";
const PROBE_SKIP_TTL_MS = 24 * 60 * 60 * 1000;

function loadProbeSkips() {
  try {
    const parsed = JSON.parse(sessionStorage.getItem(PROBE_SKIP_KEY));
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function shouldSkipProbe(host) {
  const expiry = loadProbeSkips()[host];
  return typeof expiry === "number" && Date.now() < expiry;
}

function rememberProbeOutcome(host, reachable, sameOriginOK) {
  const skips = loadProbeSkips();
  if (reachable) {
    if (!(host in skips)) return;
    delete skips[host];
  } else if (sameOriginOK) {
    skips[host] = Date.now() + PROBE_SKIP_TTL_MS;
  } else {
    // The server itself was unreachable: probably offline, not a missing
    // probe host, so keep probing on the next load.
    return;
  }
  try {
    sessionStorage.setItem(PROBE_SKIP_KEY, JSON.stringify(skips));
  } catch {
    // Storage unavailable: the probe stays eager on every load.
  }
}

function isIdle() {
  return !state.isRunning && state.phase === "idle";
}

export function resolveServerName() {
  return normalizeServerName(state.serverName);
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
    elements.startBtnHint.textContent = t(
      online ? "test.readyHint" : "test.offlineHint",
    );
  }
}

export function renderNetworkState() {
  if (elements.serverText) {
    elements.serverText.textContent = t(`server.${state.serverStatus}`);
  }
  if (state.serverStatus === "connecting") {
    if (elements.startBtn) elements.startBtn.disabled = true;
    if (elements.startBtnHint) {
      elements.startBtnHint.textContent = t("test.connecting");
    }
  } else {
    setStartAvailability(state.serverStatus === "ready");
  }
  updateNetworkDisplay();
}

function setServerOnlineUI() {
  state.serverStatus = "ready";
  if (elements.serverDot) {
    elements.serverDot.classList.remove("error");
    elements.serverDot.classList.add("connected");
  }
  renderNetworkState();
}

function setServerOfflineUI() {
  state.serverStatus = "offline";
  if (elements.serverDot) {
    elements.serverDot.classList.remove("connected");
    elements.serverDot.classList.add("error");
  }
  renderNetworkState();
}

function startsWithDigit(value) {
  if (!value || typeof value !== "string") return false;
  const code = value.codePointAt(0);
  return typeof code === "number" && code >= 48 && code <= 57;
}

export function updateNetworkDisplay() {
  const pending = t(
    state.networkInfo.complete ? "network.notDetected" : "network.detecting",
  );
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

async function discoverAddress(
  url,
  options,
  shouldUpdate = () => true,
  includeServerName = false,
) {
  try {
    const response = await fetchWithTimeout(
      url,
      options,
      TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
    );
    const data = await parseJSONOrThrow(response);
    if (includeServerName) setServerName(data?.server_name);
    if (data.client_ip && shouldUpdate()) {
      const family = data.client_ip.includes(":") ? "ipv6" : "ipv4";
      state.networkInfo[family] = data.client_ip;
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
    `${getApiBase()}/ping?meta=1`,
    { cache: "no-store" },
    canUpdateStartupAddress,
    true,
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

  const subdomainProbes = [];
  if (canProbe) {
    for (const host of [`v4.${hostname}`, `v6.${hostname}`]) {
      if (shouldSkipProbe(host)) continue;
      subdomainProbes.push(
        discoverAddress(
          `${proto}//${host}/api/v1/ping`,
          probeOpts,
          canUpdateStartupAddress,
        ).then((reachable) => ({ host, reachable })),
      );
    }
    probes.push(...subdomainProbes);
  }

  return Promise.allSettled(probes).then(async (results) => {
    const sameOriginOK = await sameOriginProbe.catch(() => false);
    for (const settled of await Promise.allSettled(subdomainProbes)) {
      if (settled.status === "fulfilled") {
        rememberProbeOutcome(
          settled.value.host,
          settled.value.reachable,
          sameOriginOK,
        );
      }
    }
    state.networkInfo.complete = true;
    updateNetworkDisplay();
    return results;
  });
}
