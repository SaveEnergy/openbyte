/** Remote health probe used by `network.js` / `checkServer`. */

import { fetchWithTimeout } from "./utils.js";
import { TEST_CONFIG } from "./state.js";

export async function isHealthyServerCandidate(url) {
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
