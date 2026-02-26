/** Latency measurement and loaded-latency probe. */

import { apiBase, state, TEST_CONFIG } from "./state.js";
import { sleep } from "./utils.js";
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

export function startLoadedLatencyProbe(signal) {
  const samples = [];
  let running = true;

  const loop = async () => {
    while (running && !signal.aborted) {
      const start = performance.now();
      try {
        const res = await fetch(`${apiBase}/ping`, {
          method: "GET",
          cache: "no-store",
          signal,
        });
        samples.push(performance.now() - start);
        await res.text().catch(() => {});
      } catch (err) {
        console.debug("loaded latency probe ping failed", err);
        if (!running) break;
      }
      await sleep(TEST_CONFIG.LOADED_LATENCY_POLL_MS);
    }
  };

  const promise = loop();

  return {
    stop() {
      running = false;
      return promise;
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
  let capturedIP = false;

  for (let i = 0; i < numSamples; i++) {
    if (signal.aborted) break;

    const start = performance.now();
    try {
      const res = await fetch(`${apiBase}/ping`, { method: "GET", signal });
      const rtt = performance.now() - start;

      if (!capturedIP && res.ok) {
        try {
          const data = await res.json();
          if (data.client_ip) {
            if (data.ipv6) {
              state.networkInfo.ipv6 = data.client_ip;
            } else {
              state.networkInfo.ipv4 = data.client_ip;
            }
            updateNetworkDisplay();
            capturedIP = true;
          }
        } catch (err) {
          console.debug("failed to parse ping response", err);
          await res.text().catch(() => {});
        }
      } else {
        await res.text().catch(() => {});
      }

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

  if (filtered.length >= 2) {
    let sumDiff = 0;
    for (let i = 1; i < filtered.length; i++) {
      sumDiff += Math.abs(filtered[i] - filtered[i - 1]);
    }
    state.jitterResult = sumDiff / (filtered.length - 1);
  } else {
    state.jitterResult = 0;
  }

  filtered.sort((a, b) => a - b);
  const median = filtered[Math.floor(filtered.length / 2)];

  return median;
}
