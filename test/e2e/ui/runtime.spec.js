const { test, expect } = require("@playwright/test");

test.describe("browser speed-test runtime", () => {
  test("shows the client IP eagerly when version metadata fails", async ({
    page,
  }) => {
    let pingRequests = 0;
    await page.route("**/api/v1/version", async (route) => {
      await route.fulfill({ status: 429, body: "rate limited" });
    });
    await page.route("**/api/v1/ping", async (route) => {
      pingRequests += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          pong: true,
          client_ip: "198.51.100.42",
          ipv6: false,
        }),
      });
    });

    await page.goto("/");

    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#idleNetworkIPv4")).toBeVisible();
    await expect(page.locator("#idleNetworkIPv4")).toHaveText(
      "198.51.100.42",
    );
    await expect(page.locator("#idleNetworkIPv6")).toHaveText("Not detected");
    await expect(page.locator("#idleNetworkInfo")).toHaveAttribute(
      "aria-busy",
      "false",
    );
    await expect(page.locator("#serverInfo")).toContainText("Ready");
    expect(pingRequests).toBeGreaterThan(0);
  });

  test("adaptive ramp respects the HTTP/1 stream cap", async ({ page }) => {
    await page.goto("/");

    const result = await page.evaluate(async () => {
      const { runAdaptiveHTTPTest } = await import("/speedtest-adaptive.js");
      const windows = [];
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
      });
      return {
        mbps,
        rampStreams: windows
          .filter((window) => window.isRamp)
          .map((window) => window.streams),
        measuredStreams: windows.at(-1).streams,
      };
    });

    expect(result.rampStreams).toEqual([1, 2, 4, 6]);
    expect(result.measuredStreams).toBe(6);
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
    await expect(page.locator("#errorMessage")).toContainText(
      /worker boot failed|worker failed/i,
    );
    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#startBtn")).toBeFocused();
  });

  test("cancel terminates an active worker download", async ({ page }) => {
    await page.addInitScript(() => {
      const OriginalWorker = globalThis.Worker;
      globalThis.__openbyteWorkerTerminated = false;
      globalThis.Worker = class TrackedWorker extends OriginalWorker {
        terminate() {
          globalThis.__openbyteWorkerTerminated = true;
          super.terminate();
        }
      };
    });
    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");

    const downloadStarted = page.waitForRequest((request) =>
      request.url().includes("/api/v1/download?"),
    );
    await page.locator("#startBtn").click();
    await downloadStarted;

    await page.locator("#cancelBtn").click();

    await expect
      .poll(() =>
        page.evaluate(() => globalThis.__openbyteWorkerTerminated),
      )
      .toBe(true);
    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#startBtn")).toBeFocused();
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
