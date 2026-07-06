#!/usr/bin/env node
import { chromium } from "@playwright/test";

const mode = process.argv[2] || process.env.MODE || "ui";
const runs = positiveInt(process.env.RUNS, 5);
const baseURL = process.env.URL || "http://localhost:8080/";
const maxStreams = positiveInt(process.env.MAX_STREAMS, 8);
const rampDuration = positiveInt(process.env.RAMP_DURATION, 1);
const measureDuration = positiveInt(process.env.MEASURE_DURATION, 3);
const directionDuration = positiveInt(process.env.DURATION, 3);
const ignoreHTTPSErrors = process.env.IGNORE_HTTPS_ERRORS === "1";

function positiveInt(raw, fallback) {
  const value = Number.parseInt(raw || "", 10);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function parseList(raw, fallback) {
  return (raw || fallback)
    .split(",")
    .map((part) => positiveInt(part.trim(), 0))
    .filter((value) => value > 0);
}

function median(values) {
  const sorted = values
    .filter((value) => Number.isFinite(value))
    .toSorted((a, b) => a - b);
  if (sorted.length === 0) return 0;
  return sorted[Math.floor(sorted.length / 2)];
}

function uiURL() {
  const url = new URL(baseURL);
  url.searchParams.set("maxStreams", String(maxStreams));
  url.searchParams.set("rampDuration", String(rampDuration));
  url.searchParams.set("measureDuration", String(measureDuration));
  return url.toString();
}

function toMbps(valueText, unitText) {
  const value = Number.parseFloat(valueText || "0");
  if (!Number.isFinite(value)) return 0;
  return /gbps/i.test(unitText || "") ? value * 1000 : value;
}

async function runUI(page) {
  await page.goto(uiURL());
  await page.locator("#startBtn").click();
  await page.locator("#resultsState").waitFor({ timeout: 120_000 });
  const downloadValue = (await page.locator("#downloadResult").textContent()) || "0";
  const downloadUnit =
    (await page.locator(".result-primary .result-unit").textContent()) || "Mbps";
  const uploadValue = (await page.locator("#uploadResult").textContent()) || "0";
  const uploadUnit =
    (await page.locator(".result-secondary .result-unit").textContent()) || "Mbps";
  return {
    downloadMbps: toMbps(downloadValue, downloadUnit),
    uploadMbps: toMbps(uploadValue, uploadUnit),
    raw: {
      download: `${downloadValue.trim()} ${downloadUnit.trim()}`,
      upload: `${uploadValue.trim()} ${uploadUnit.trim()}`,
    },
  };
}

async function runUploadBlobProbe(page, blobMB, streams) {
  return page.evaluate(
    async ({ blobMB, streams, duration }) => {
      const size = blobMB * 1024 * 1024;
      const piece = new Uint8Array(65536);
      crypto.getRandomValues(piece);
      const parts = Array.from({ length: Math.ceil(size / piece.length) }, () => piece);
      const blob = new Blob(parts, { type: "application/octet-stream" });
      const endAt = performance.now() + duration * 1000;
      let total = 0;
      const worker = async () => {
        while (performance.now() < endAt) {
          const res = await fetch("/api/v1/upload", {
            method: "POST",
            body: blob,
            headers: { "Content-Type": "application/octet-stream" },
          });
          await res.text();
          if (!res.ok) throw new Error(`upload status ${res.status}`);
          total += size;
        }
      };
      const started = performance.now();
      await Promise.all(Array.from({ length: streams }, () => worker()));
      const seconds = (performance.now() - started) / 1000;
      return { blobMB, streams, gbps: (total * 8) / seconds / 1e9 };
    },
    { blobMB, streams, duration: directionDuration },
  );
}

async function runStreamingUploadProbe(page, streams) {
  return page.evaluate(
    async ({ streams, duration }) => {
      const supportsRequestStreams = (() => {
        try {
          new Request("/", {
            method: "POST",
            body: new ReadableStream(),
            duplex: "half",
          });
          return true;
        } catch {
          return false;
        }
      })();
      if (!supportsRequestStreams) return { supported: false, reason: "unsupported" };

      const chunk = new Uint8Array(65536);
      crypto.getRandomValues(chunk);
      let total = 0;
      const worker = async () => {
        const started = performance.now();
        const body = new ReadableStream({
          pull(controller) {
            if (performance.now() - started >= duration * 1000) {
              controller.close();
              return;
            }
            controller.enqueue(chunk);
            total += chunk.byteLength;
          },
        });
        const res = await fetch("/api/v1/upload", {
          method: "POST",
          body,
          duplex: "half",
          headers: { "Content-Type": "application/octet-stream" },
        });
        await res.text();
        if (!res.ok) throw new Error(`upload status ${res.status}`);
      };
      const started = performance.now();
      try {
        await Promise.all(Array.from({ length: streams }, () => worker()));
      } catch (err) {
        return {
          supported: true,
          ok: false,
          reason: String(err?.message || err),
        };
      }
      const seconds = (performance.now() - started) / 1000;
      return {
        supported: true,
        ok: true,
        streams,
        gbps: (total * 8) / seconds / 1e9,
      };
    },
    { streams, duration: directionDuration },
  );
}

async function runDownloadShardProbe(context, shards) {
  const pages = await Promise.all(
    Array.from({ length: shards }, async () => {
      const page = await context.newPage();
      await page.goto(baseURL);
      return page;
    }),
  );
  try {
    const results = await Promise.all(
      pages.map((page) =>
        page.evaluate(async (duration) => {
          const res = await fetch(
            `/api/v1/download?duration=${duration}&chunk=1048576`,
          );
          const reader = res.body.getReader({ mode: "byob" });
          let buffer = new ArrayBuffer(1024 * 1024);
          let total = 0;
          const started = performance.now();
          while (true) {
            const { done, value } = await reader.read(new Uint8Array(buffer));
            if (done) break;
            buffer = value.buffer;
            total += value.byteLength;
          }
          const seconds = (performance.now() - started) / 1000;
          return (total * 8) / seconds / 1e9;
        }, directionDuration),
      ),
    );
    return { shards, gbps: results.reduce((sum, value) => sum + value, 0), parts: results };
  } finally {
    await Promise.all(pages.map((page) => page.close()));
  }
}

function printSummary(label, rows, metric) {
  const values = rows.map((row) => row[metric]);
  const summary = {
    label,
    runs: rows.length,
    median: Number(median(values).toFixed(2)),
    min: Number(Math.min(...values).toFixed(2)),
    max: Number(Math.max(...values).toFixed(2)),
  };
  console.log(JSON.stringify({ summary }));
}

const browser = await chromium.launch();
const context = await browser.newContext({ ignoreHTTPSErrors });
try {
  if (mode === "ui") {
    const page = await context.newPage();
    const rows = [];
    for (let i = 0; i < runs; i++) {
      const result = await runUI(page);
      const row = { mode, run: i + 1, ...result };
      rows.push(row);
      console.log(JSON.stringify(row));
    }
    printSummary("download_mbps", rows, "downloadMbps");
    printSummary("upload_mbps", rows, "uploadMbps");
    await page.close();
  } else if (mode === "upload-blobs") {
    const page = await context.newPage();
    await page.goto(baseURL);
    const sizes = parseList(process.env.BLOB_MB, "8,32,64");
    for (const blobMB of sizes) {
      const result = await runUploadBlobProbe(page, blobMB, maxStreams);
      console.log(JSON.stringify({ mode, ...result }));
    }
    await page.close();
  } else if (mode === "upload-stream") {
    const page = await context.newPage();
    await page.goto(baseURL);
    console.log(JSON.stringify({ mode, ...(await runStreamingUploadProbe(page, maxStreams)) }));
    await page.close();
  } else if (mode === "download-shards") {
    const shardsList = parseList(process.env.SHARDS, "1,2,4");
    for (const shards of shardsList) {
      console.log(JSON.stringify({ mode, ...(await runDownloadShardProbe(context, shards)) }));
    }
  } else {
    throw new Error(`unknown mode: ${mode}`);
  }
} finally {
  await context.close();
  await browser.close();
}
