/** Worker data plane for browser HTTP speed tests. */

import { state, TEST_CONFIG } from "./state.js";
import { runDownloadTest, runUploadTest } from "./speedtest-http.js";

let currentRun = null;

function post(id, type, payload = {}) {
  globalThis.postMessage({ id, type, ...payload });
}

function serializeError(error) {
  return {
    name: error?.name || "Error",
    message: error?.message || "Speed test failed",
  };
}

function createProgressReporter(id) {
  let lastPost = 0;
  let lastSeenBytes = 0;

  return (bytes) => {
    const now = performance.now();
    const reset = bytes < lastSeenBytes;
    lastSeenBytes = bytes;

    if (reset || now - lastPost >= TEST_CONFIG.PROGRESS_TICK_MS) {
      lastPost = now;
      post(id, "progress", { bytes });
    }
  };
}

function runByDirection(direction, onProgress, signal, options) {
  if (direction === "download") {
    return runDownloadTest(onProgress, signal, options);
  }
  if (direction === "upload") {
    return runUploadTest(onProgress, signal, options);
  }
  throw new Error(`Unknown speed test direction: ${direction}`);
}

async function runSpeedTest({ id, direction, config }) {
  if (currentRun) currentRun.controller.abort();

  const controller = new AbortController();
  currentRun = { id, controller };
  state.diagnostics = null;

  try {
    const mbps = await runByDirection(
      direction,
      createProgressReporter(id),
      controller.signal,
      {
        config,
        onPhase: (stage, streams) => post(id, "phase", { stage, streams }),
        onMeasureStart: (streams) => post(id, "measureStart", { streams }),
      },
    );
    post(id, "result", { mbps, diagnostics: state.diagnostics });
  } catch (error) {
    post(id, "error", serializeError(error));
  } finally {
    if (currentRun?.id === id) currentRun = null;
  }
}

function isTrustedMessageOrigin(event) {
  // Dedicated worker messages are same-origin by construction, but Chromium
  // reports an empty origin for parent→worker messages. Keep the explicit
  // origin gate for browsers that populate it, and only allow the empty worker
  // origin fallback.
  return event.origin === "" || event.origin === globalThis.location.origin;
}

globalThis.addEventListener("message", (event) => {
  if (!isTrustedMessageOrigin(event)) return;

  const message = event.data || {};

  if (message.type === "cancel") {
    if (currentRun?.id === message.id) currentRun.controller.abort();
    return;
  }

  if (message.type === "run") {
    void runSpeedTest(message);
  }
});
