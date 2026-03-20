/** Client IP discovery via main + split-horizon v4/v6 host probes. */

import { getApiBase, state, elements } from "./state.js";
import { parseJSONOrThrow } from "./utils.js";
import { startsWithDigit } from "./network-helpers.js";

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

  Promise.allSettled([mainPing, v4Ping, v6Ping]).then(() =>
    updateNetworkDisplay(),
  );
}
