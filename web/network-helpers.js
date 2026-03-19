/** URL/string helpers and server scoring used by `network.js`. */

export function trimTrailingSlashes(value) {
  if (typeof value !== "string" || value.length === 0) return value;
  let end = value.length;
  while (end > 0 && value.codePointAt(end - 1) === 47) {
    end -= 1;
  }
  return value.slice(0, end);
}

export function startsWithDigit(value) {
  if (!value || typeof value !== "string") return false;
  const code = value.codePointAt(0);
  if (typeof code !== "number") return false;
  return code >= 48 && code <= 57;
}

export function loadServersErrorMessage(err) {
  if (err?.name === "AbortError") {
    return "Timed out while loading servers";
  }
  if (typeof err?.message === "string" && err.message.trim() !== "") {
    return err.message;
  }
  return "Failed to load servers";
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

export function serverLoadFactor(server) {
  const max = Math.max(1, server.max_tests ?? 1);
  const active = server.active_tests ?? 0;
  return 1 + 0.3 * (active / max);
}
