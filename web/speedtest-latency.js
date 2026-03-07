/** Latency measurement and loaded-latency probe. */

import { getApiBase, state, TEST_CONFIG } from "./state.js";
import { fetchWithTimeout } from "./utils.js";
import { updateNetworkDisplay } from "./network.js";
import { updateProgress, updateSpeed } from "./ui.js";

function filterOutliersIQR(samples) {
  if (samples.length < 4) return samples.slice();
  const sorted = samples.slice().sort((a, b) => a - b);
  const q1 = sorted[Math.floor(sorted.length * 0.25)];
  const q3 = sorted[Math.floor(sorted.length * 0.75)];
  const iqr = q3 - q1;
  const lower = q1 - 1.5 * iqr;
  const upper = q3 + 1.5 * iqr;
  return samples.filter((s) => s >= lower && s <= upper);
}

async function captureClientIPIfNeeded(res, capturedRef) {
  if (capturedRef.captured || !res.ok) {
    await res.text().catch(() => {});
    return;
  }
  try {
    const data = await res.json();
    if (data.client_ip) {
      state.networkInfo[data.ipv6 ? "ipv6" : "ipv4"] = data.client_ip;
      updateNetworkDisplay();
      capturedRef.captured = true;
    }
  } catch (err) {
    console.debug("failed to parse ping response", err);
    await res.text().catch(() => {});
  }
}

function computeJitter(samples) {
  if (samples.length < 2) return 0;
  let sumDiff = 0;
  for (let i = 1; i < samples.length; i++) {
    sumDiff += Math.abs(samples[i] - samples[i - 1]);
  }
  return sumDiff / (samples.length - 1);
}

function sleepWithSignal(ms, signal) {
  if (ms <= 0 || !signal) return Promise.resolve();
  if (signal.aborted) return Promise.resolve();
  return new Promise((resolve) => {
    let settled = false;
    const complete = () => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      signal.removeEventListener("abort", complete);
      resolve();
    };
    const timer = setTimeout(complete, ms);
    signal.addEventListener("abort", complete, { once: true });
  });
}

export function startLoadedLatencyProbe(signal) {
  const samples = [];
  let running = true;
  const probeController = new AbortController();
  const onAbort = () => probeController.abort();
  if (signal.aborted) {
    probeController.abort();
  } else {
    signal.addEventListener("abort", onAbort, { once: true });
  }

  const loop = async () => {
    try {
      while (running && !signal.aborted) {
        const start = performance.now();
        try {
          const res = await fetchWithTimeout(
            `${getApiBase()}/ping`,
            {
              method: "GET",
              cache: "no-store",
              signal: probeController.signal,
            },
            TEST_CONFIG.HEALTH_CHECK_TIMEOUT_MS,
          );
          samples.push(performance.now() - start);
          await res.text().catch(() => {});
        } catch (err) {
          console.debug("loaded latency probe ping failed", err);
          if (!running || signal.aborted || probeController.signal.aborted)
            break;
        }
        await sleepWithSignal(
          TEST_CONFIG.LOADED_LATENCY_POLL_MS,
          probeController.signal,
        );
      }
    } finally {
      signal.removeEventListener("abort", onAbort);
    }
  };

  const promise = loop();

  return {
    async stop() {
      running = false;
      probeController.abort();
      await promise;
    },
    getMedian() {
      if (samples.length === 0) return 0;
      const filtered = filterOutliersIQR(samples);
      if (filtered.length === 0) return 0;
      filtered.sort((a, b) => a - b);
      return filtered[Math.floor(filtered.length / 2)];
    },
  };
}

export async function measureLatency(signal) {
  const rawSamples = [];
  const numSamples = TEST_CONFIG.LATENCY_SAMPLE_COUNT;
  const warmUpPings = TEST_CONFIG.LATENCY_WARMUP_PINGS;
  const capturedRef = { captured: false };

  for (let i = 0; i < numSamples; i++) {
    if (signal.aborted) break;

    const start = performance.now();
    try {
      const res = await fetch(`${getApiBase()}/ping`, {
        method: "GET",
        signal,
      });
      const rtt = performance.now() - start;

      await captureClientIPIfNeeded(res, capturedRef);

      rawSamples.push(rtt);
      updateProgress((i / numSamples) * 100);
      updateSpeed(rtt, "latency");
    } catch (e) {
      if (e.name === "AbortError") break;
    }
  }

  if (rawSamples.length === 0) return null;

  const samples =
    rawSamples.length > warmUpPings
      ? rawSamples.slice(warmUpPings)
      : rawSamples;

  const filtered = filterOutliersIQR(samples);
  state.jitterResult = computeJitter(filtered);

  filtered.sort((a, b) => a - b);
  return filtered[Math.floor(filtered.length / 2)];
}
