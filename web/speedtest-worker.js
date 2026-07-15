/** One-shot worker data plane for one browser HTTP speed-test direction. */

import { TEST_CONFIG } from "./state.js";

function post(type, payload = {}) {
  globalThis.postMessage({ type, ...payload });
}

function serializeError(error) {
  return {
    name: error?.name || "Error",
    code: error?.code || "worker.failed",
  };
}

function createProgressReporter() {
  let lastPost = 0;
  let lastSeenBytes = 0;

  return (bytes) => {
    const now = performance.now();
    const reset = bytes < lastSeenBytes;
    lastSeenBytes = bytes;
    if (reset || now - lastPost >= TEST_CONFIG.PROGRESS_TICK_MS) {
      lastPost = now;
      post("progress", { bytes });
    }
  };
}

async function loadDirection(direction) {
  if (direction === "download") {
    return (await import("./speedtest-http-download.js")).runDownloadTest;
  }
  if (direction === "upload") {
    return (await import("./speedtest-http-upload.js")).runUploadTest;
  }
  throw new Error(`Unknown speed test direction: ${direction}`);
}

async function runSpeedTest({ direction, config }) {
  try {
    const run = await loadDirection(direction);
    const mbps = await run(
      createProgressReporter(),
      new AbortController().signal,
      {
        config,
        onPhase: (stage, streams, info) =>
          post("phase", { stage, streams, ...info }),
        onMeasureStart: (streams, duration) =>
          post("measureStart", { streams, duration }),
      },
    );
    post("result", { mbps });
  } catch (error) {
    post("error", serializeError(error));
  }
}

globalThis.addEventListener(
  "message",
  (event) => {
    if (event.origin !== "" && event.origin !== globalThis.location.origin) {
      return;
    }
    void runSpeedTest(event.data || {});
  },
  { once: true },
);
