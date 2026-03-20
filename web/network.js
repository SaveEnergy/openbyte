/** Server discovery, health checks, and network info (barrel). */

export { getHealthURL } from "./network-helpers.js";
export { detectNetworkInfo, updateNetworkDisplay } from "./network-probes.js";
export {
  resolveServerName,
  setSelectedServer,
  updateServerName,
  loadServers,
  selectFastestServer,
  populateServerSelect,
  onServerChange,
  checkServer,
} from "./network-servers.js";
