const { test, expect } = require("@playwright/test");

test.describe("browser speed-test runtime", () => {
  test("discovers the client IP eagerly", async ({ page }) => {
    let pingRequests = 0;
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

    await expect(page.locator("#networkIPv4")).toHaveText("198.51.100.42");
    await expect(page.locator("#idleState")).toBeVisible();
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
