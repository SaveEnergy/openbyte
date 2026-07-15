const { test, expect } = require("@playwright/test");
const http = require("node:http");

test.describe("browser speed-test runtime", () => {
  test("shows the client IP eagerly from bootstrap ping", async ({ page }) => {
    let pingRequests = 0;
    await page.route("**/api/v1/ping*", async (route) => {
      pingRequests += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          client_ip: "198.51.100.42",
          server_name: "Playwright Server",
        }),
      });
    });

    await page.goto("/");

    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#idleNetworkIPv4")).toBeVisible();
    await expect(page.locator("#idleNetworkIPv4")).toHaveText(
      "198.51.100.42",
    );
    await expect(page.locator("#idleNetworkIPv6")).toHaveText("Not available");
    await expect(page.locator("#idleNetworkInfo")).toHaveAttribute(
      "aria-busy",
      "false",
    );
    await expect(page.locator("#serverInfo")).toContainText("Ready");
    await expect(page.locator("#serverName")).toHaveText("Playwright Server");
    expect(pingRequests).toBeGreaterThan(0);
  });

  test("adaptive ramp respects the HTTP/1 stream cap", async ({ page }) => {
    await page.goto("/");

    const result = await page.evaluate(async () => {
      const { runAdaptiveHTTPTest } = await import("/speedtest-adaptive.js");
      const windows = [];
      let maxWindows;
      const controller = new AbortController();
      const mbps = await runAdaptiveHTTPTest({
        signal: controller.signal,
        config: {
          rampDuration: 1,
          measureDuration: 1,
          measureDurationOverridden: true,
          maxStreams: 64,
          gainThreshold: 0.08,
          nextHopProtocol: "http/1.1",
        },
        runWindow: async (options) => {
          windows.push(options);
          return options.streams * 100;
        },
        onPhase: (_stage, _streams, info) => {
          maxWindows = info.maxWindows;
        },
      });
      return {
        mbps,
        rampStreams: windows
          .filter((window) => window.isRamp)
          .map((window) => window.streams),
        measuredStreams: windows.at(-1).streams,
        maxWindows,
      };
    });

    expect(result.rampStreams).toEqual([1, 2, 4, 6]);
    expect(result.measuredStreams).toBe(6);
    expect(result.maxWindows).toBe(4);
    expect(result.mbps).toBe(600);
  });

  test("download retries exhaust into a useful error", async ({ page }) => {
    let downloadRequests = 0;
    await page.route("**/api/v1/download?**", async (route) => {
      downloadRequests += 1;
      await route.abort("failed");
    });

    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
    await page.locator("#startBtn").click();

    await expect(page.locator("#errorToast")).toBeVisible({ timeout: 20_000 });
    await expect(page.locator("#errorMessage")).toContainText(/network error/i);
    await expect(page.locator("#idleState")).toBeVisible();
    expect(downloadRequests).toBeGreaterThan(1);
  });

  test("worker startup failure returns to an actionable idle state", async ({
    page,
  }) => {
    await page.route("**/speedtest-worker.js", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/javascript",
        body: 'throw new Error("worker boot failed");',
      });
    });

    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
    await page.locator("#startBtn").click();

    await expect(page.locator("#errorToast")).toBeVisible({ timeout: 20_000 });
    await expect(page.locator("#errorMessage")).toHaveText(
      "The speed test stopped. Please try again.",
    );
    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#startBtn")).toBeFocused();
  });

  test("cancel aborts an active worker download", async ({ page }) => {
    let active = false;
    let aborted = false;
    const server = http.createServer((request, response) => {
      active = true;
      request.once("aborted", () => {
        aborted = true;
      });
      response.once("close", () => {
        if (!response.writableEnded) aborted = true;
      });
      // Keep the response pending so cancel must tear down a live connection.
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });

    const pattern = "**/api/v1/download?**";
    const { port } = server.address();
    try {
      await page.addInitScript(() => {
        const OriginalWorker = globalThis.Worker;
        globalThis.__workerLifecycle = { created: 0, terminated: 0 };
        globalThis.Worker = class TrackedWorker extends OriginalWorker {
          constructor(...args) {
            super(...args);
            globalThis.__workerLifecycle.created += 1;
          }

          terminate() {
            globalThis.__workerLifecycle.terminated += 1;
            return super.terminate();
          }
        };
      });
      await page.route(pattern, async (route) => {
        const url = new URL(route.request().url());
        await route.continue({
          url: `http://127.0.0.1:${port}${url.pathname}${url.search}`,
        });
      });

      await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
      await page.locator("#startBtn").click();

      await expect.poll(() => active, { timeout: 15_000 }).toBe(true);
      expect(
        await page.evaluate(() => globalThis.__workerLifecycle),
      ).toEqual({ created: 1, terminated: 0 });

      await page.locator("#cancelBtn").click();

      await expect.poll(() => aborted).toBe(true);
      await expect
        .poll(() =>
          page.evaluate(() => globalThis.__workerLifecycle.terminated),
        )
        .toBe(1);
      await expect(page.locator("#idleState")).toBeVisible();
      await expect(page.locator("#startBtn")).toBeFocused();
    } finally {
      await page.unroute(pattern);
      await new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      });
    }
  });

  test("reduced motion keeps measurement status static", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
    await page.locator("#startBtn").click();
    await expect(page.locator("#testingState")).toBeVisible();

    const animationName = await page.locator(".instrument-ring-arc").evaluate(
      (element) => getComputedStyle(element).animationName,
    );
    expect(animationName).toBe("none");
    await page.locator("#cancelBtn").click();
  });
});
