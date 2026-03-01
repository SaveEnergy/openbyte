/** Server discovery, health checks, and network info. */

import {
  getApiBase,
  state,
  setApiBase,
  elements,
  TEST_CONFIG,
} from "./state.js";
import {
  parseJSONOrThrow,
  isSameOriginURL,
  fetchWithTimeout,
} from "./utils.js";

function trimTrailingSlashes(value) {
  if (typeof value !== "string" || value.length === 0) return value;
  let end = value.length;
  while (end > 0 && value.codePointAt(end - 1) === 47) {
    end -= 1;
  }
  return value.slice(0, end);
}

function startsWithDigit(value) {
  if (!value || typeof value !== "string") return false;
  const code = value.codePointAt(0);
  if (typeof code !== "number") return false;
  return code >= 48 && code <= 57;
}

export function resolveServerName() {
  if (state.selectedServer?.name) return state.selectedServer.name;
  if (state.servers?.length) {
    const fallback = state.servers[0]?.name;
    if (fallback) return fallback;
  }
  return "Current Server";
}

export function setSelectedServer(server) {
  state.selectedServer = server || null;
  if (state.selectedServer?.api_endpoint) {
    setApiBase(`${state.selectedServer.api_endpoint}/api/v1`);
  } else {
    setApiBase("/api/v1");
  }
}

export function getHealthURL(server) {
  if (server.api_endpoint) {
    try {
      const apiURL = new URL(server.api_endpoint);
      if (
        globalThis.location.protocol === "https:" &&
        apiURL.protocol === "http:"
      ) {
        apiURL.protocol = "https:";
      }
      apiURL.pathname = trimTrailingSlashes(apiURL.pathname) + "/health";
      return apiURL.toString();
    } catch (e) {
      console.debug("failed to parse server api_endpoint", e);
      return `${server.api_endpoint}/health`;
    }
  }
  const protocol = globalThis.location.protocol || "http:";
  return `${protocol}//${server.host}/health`;
}

export function updateNetworkDisplay() {
  if (elements.networkIPv4) {
    elements.networkIPv4.textContent = state.networkInfo.ipv4 || "-";
  }
  if (elements.networkIPv6) {
    elements.networkIPv6.textContent = state.networkInfo.ipv6 || "-";
  }
}

export function updateServerName() {
  if (elements.serverName) {
    elements.serverName.textContent = resolveServerName();
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

  Promise.allSettled([mainPing, v4Ping, v6Ping]).then(() =>
    updateNetworkDisplay(),
  );
}

export async function loadServers() {
  try {
    const res = await fetch(`${getApiBase()}/servers`);
    if (!res.ok) {
      await res.text().catch(() => {});
      throw new Error(`Failed to load servers: HTTP ${res.status}`);
    }
    let data;
    try {
      data = await res.json();
    } catch (err) {
      state.servers = [];
      throw new Error("Failed to parse servers response", { cause: err });
    }
    state.servers = Array.isArray(data.servers) ? data.servers : [];

    if (state.servers.length > 0) {
      setSelectedServer(state.servers[0]);
      updateServerName();
      await selectFastestServer();
    } else {
      state.selectedServer = null;
    }

    populateServerSelect();
    updateServerName();
    checkServer();
  } catch (e) {
    console.error("Failed to load servers:", e);
    state.servers = [];
    setSelectedServer(null);
    populateServerSelect();
    updateServerName();
    checkServer();
  }
}

export async function selectFastestServer() {
  if (state.servers.length === 0) {
    checkServer();
    return;
  }

  if (elements.serverText)
    elements.serverText.textContent = "Finding fastest...";

  const latencyPromises = state.servers.map(async (server) => {
    const healthUrl = getHealthURL(server);
    const isSameOrigin = isSameOriginURL(healthUrl);

    const start = performance.now();
    try {
      const res = await fetchWithTimeout(
        healthUrl,
        {
          method: "GET",
          mode: isSameOrigin ? "same-origin" : "cors",
        },
        TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
      );
      const latency = performance.now() - start;

      if (res.ok) {
        await res.text().catch(() => {});
        return { server, latency, error: null };
      }
      await res.text().catch(() => {});
      return { server, latency: Infinity, error: "unhealthy" };
    } catch (e) {
      return { server, latency: Infinity, error: e.message };
    }
  });

  const results = await Promise.all(latencyPromises);

  results.forEach(({ server, latency, error }) => {
    server.reachable = error === null;
    server.latency = error === null ? latency : null;
  });

  const reachable = results.filter((r) => r.error === null);

  if (reachable.length === 0) {
    console.warn("No servers reachable, defaulting to current");
    setSelectedServer(null);
    updateServerName();
    return;
  }

  const loadFactor = (s) => {
    const max = Math.max(1, s.max_tests ?? 1);
    const active = s.active_tests ?? 0;
    return 1 + 0.3 * (active / max);
  };
  reachable.sort((a, b) => {
    const scoreA = a.latency * loadFactor(a.server);
    const scoreB = b.latency * loadFactor(b.server);
    return scoreA - scoreB;
  });
  setSelectedServer(reachable[0].server);
  updateServerName();

  console.log(
    "Auto-selected server:",
    state.selectedServer.name,
    `(${Math.round(reachable[0].latency)}ms)`,
  );
}

export function populateServerSelect() {
  if (!elements.serverSelect || !elements.serverSelectGroup) return;

  while (elements.serverSelect.firstChild)
    elements.serverSelect.firstChild.remove();

  const reachableServers = state.servers.filter(
    (server) => server.api_endpoint && server.reachable,
  );

  if (reachableServers.length <= 1) {
    elements.serverSelectGroup.classList.add("hidden");
    if (
      reachableServers.length === 1 &&
      (!state.selectedServer ||
        state.selectedServer.id !== reachableServers[0].id)
    ) {
      setSelectedServer(reachableServers[0]);
      updateServerName();
    }
    return;
  }

  elements.serverSelectGroup.classList.remove("hidden");
  reachableServers.forEach((server) => {
    const opt = document.createElement("option");
    opt.value = server.id;
    const location = server.location ? ` (${server.location})` : "";
    opt.textContent = `${server.name}${location}`;
    elements.serverSelect.appendChild(opt);
  });

  if (
    state.selectedServer?.id &&
    reachableServers.some((s) => s.id === state.selectedServer.id)
  ) {
    elements.serverSelect.value = state.selectedServer.id;
  } else if (reachableServers[0]?.id) {
    setSelectedServer(reachableServers[0]);
    elements.serverSelect.value = reachableServers[0].id;
    updateServerName();
  }
}

export function onServerChange() {
  const value = elements.serverSelect.value;
  const server = state.servers.find((s) => s.id === value && s.api_endpoint);
  if (server) setSelectedServer(server);
  updateServerName();
}

function setServerOnlineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("error", "warning");
    elements.serverDot.classList.add("connected");
  }

  if (state.selectedServer) {
    if (elements.serverText)
      elements.serverText.textContent = state.selectedServer.name || "Ready";
  } else if (elements.serverText) {
    elements.serverText.textContent = "Ready";
  }

  if (elements.serverStatus) {
    elements.serverStatus.textContent = "Connected";
    elements.serverStatus.className = "server-status connected";
  }
}

function setServerOfflineUI() {
  if (elements.serverDot) {
    elements.serverDot.classList.remove("connected", "warning");
    elements.serverDot.classList.add("error");
  }
  if (elements.serverText) elements.serverText.textContent = "Offline";
  if (elements.serverStatus) {
    elements.serverStatus.textContent = "Offline";
    elements.serverStatus.className = "server-status error";
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

export async function checkServer() {
  const candidates = [
    "/health",
    `${getApiBase()}/health`,
    `${getApiBase()}/ping`,
  ];

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
