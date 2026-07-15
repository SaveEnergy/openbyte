const { test, expect } = require("@playwright/test");
const pingRoute = /\/api\/v1\/ping(?:\?.*)?$/;

test.describe("openByte UI regressions", () => {
  test("keeps GO disabled until the server probe succeeds", async ({ page }) => {
    await page.route(pingRoute, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 500));
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: "offline" }),
      });
    });

    await page.goto("/");
    await expect(page.locator("#startBtn")).toBeDisabled();
    await expect(page.locator("#startBtn")).toContainText("Connecting");
    await expect(page.locator("#startBtn")).toContainText("retrying");

    await page.evaluate(() => {
      const button = document.getElementById("startBtn");
      button.disabled = false;
      button.click();
    });
    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#testingState")).toBeHidden();
  });

  test("does not refresh a completed result's public IP", async ({ page }) => {
    await page.goto("/");
    await page.route(pingRoute, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ client_ip: "203.0.113.99" }),
      });
    });

    const networkInfo = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      const { checkServer, updateNetworkDisplay } = await import("/network.js");
      state.phase = "results";
      state.isRunning = false;
      state.networkInfo.ipv4 = "198.51.100.42";
      state.networkInfo.complete = true;
      updateNetworkDisplay();
      await checkServer();
      return {
        ipv4: state.networkInfo.ipv4,
        rendered: document.getElementById("networkIPv4").textContent,
      };
    });

    expect(networkInfo).toEqual({
      ipv4: "198.51.100.42",
      rendered: "198.51.100.42",
    });
  });

  test("an in-flight readiness probe can disable the next run", async ({
    page,
  }) => {
    let probeStarted = false;
    let releaseProbe;
    const probeGate = new Promise((resolve) => {
      releaseProbe = resolve;
    });
    await page.goto("/");
    await page.route(pingRoute, async (route) => {
      probeStarted = true;
      await probeGate;
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ error: "offline" }),
      });
    });

    await page.evaluate(async () => {
      const { state } = await import("/state.js");
      const { checkServer } = await import("/network.js");
      state.phase = "idle";
      state.isRunning = false;
      globalThis.__inFlightReadiness = checkServer();
    });
    await expect.poll(() => probeStarted).toBe(true);
    await page.evaluate(async () => {
      const { state } = await import("/state.js");
      state.phase = "latency";
      state.isRunning = true;
    });
    releaseProbe();
    await page.evaluate(() => globalThis.__inFlightReadiness);

    const readiness = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      return state.serverOnline;
    });
    expect(readiness).toBe(false);
    await expect(page.locator("#startBtn")).toBeDisabled();
  });

  test("delayed startup probes can finish during the first test", async ({
    page,
    baseURL,
  }) => {
    await page.route(pingRoute, async (route) => {
      const hostname = new URL(route.request().url()).hostname;
      const isCrossProbe = hostname.startsWith("v4.") || hostname.startsWith("v6.");
      if (isCrossProbe) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
      await route.fulfill({
        status: 200,
        headers: { "Access-Control-Allow-Origin": "*" },
        contentType: "application/json",
        body: JSON.stringify({
          client_ip: hostname.startsWith("v6.")
            ? "2001:db8::99"
            : "198.51.100.99",
        }),
      });
    });
    const testURL = new URL(
      "/?maxStreams=1&measureDuration=1&rampDuration=1",
      baseURL,
    );
    testURL.hostname = "openbyte.localhost";
    await page.goto(testURL.href);
    await page.locator("#startBtn").click();
    await expect(page.locator("#testingState")).toBeVisible();
    await expect
      .poll(() =>
        page.evaluate(async () => (await import("/state.js")).state.networkInfo.ipv6),
      )
      .toBe("2001:db8::99");
    await page.locator("#cancelBtn").click();
  });

  test("theme toggle cycles when storage is unavailable", async ({ page }) => {
    await page.addInitScript(() => {
      for (const method of ["getItem", "setItem", "removeItem"]) {
        Object.defineProperty(Storage.prototype, method, {
          configurable: true,
          value() {
            throw new DOMException("Storage blocked", "SecurityError");
          },
        });
      }
    });
    await page.goto("/");
    const html = page.locator("html");

    await page.locator("#themeToggle").click();
    await expect(html).toHaveAttribute("data-theme", "light");
    await page.locator("#themeToggle").click();
    await expect(html).toHaveAttribute("data-theme", "dark");
    await page.locator("#themeToggle").click();
    await expect(html).not.toHaveAttribute("data-theme", /.+/);
  });

  test("capped-ramp progress stays truthful", async ({ page }) => {
    await page.goto("/");
    const result = await page.evaluate(async () => {
      const { runAdaptiveHTTPTest } = await import("/speedtest-adaptive.js");
      const phases = [];
      await runAdaptiveHTTPTest({
        signal: new AbortController().signal,
        config: {
          maxStreams: 6,
          rampDuration: 1,
          measureDuration: 1,
          measureDurationOverridden: true,
          gainThreshold: 0.08,
          nextHopProtocol: "http/1.1",
        },
        runWindow: async ({ streams }) => streams * 100,
        onPhase: (stage, streams, info) => {
          if (stage === "saturating") {
            phases.push({ streams, maxWindows: info.maxWindows });
          }
        },
      });
      return phases;
    });

    expect(result).toEqual([
      { streams: 1, maxWindows: 4 },
      { streams: 2, maxWindows: 4 },
      { streams: 4, maxWindows: 4 },
      { streams: 6, maxWindows: 4 },
    ]);
  });

  test("one-tap share falls back when gesture-gated APIs reject", async ({
    page,
    baseURL,
  }) => {
    let saveRequests = 0;
    await page.addInitScript(() => {
      globalThis.__shareFallback = { clipboard: 0, share: 0, prompts: [] };
      Object.defineProperty(navigator, "clipboard", {
        configurable: true,
        value: {
          async writeText() {
            globalThis.__shareFallback.clipboard += 1;
            throw new DOMException("Clipboard blocked", "NotAllowedError");
          },
        },
      });
      Object.defineProperty(navigator, "share", {
        configurable: true,
        value: async () => {
          globalThis.__shareFallback.share += 1;
          throw new DOMException("Activation expired", "NotAllowedError");
        },
      });
      globalThis.prompt = (message, value) => {
        globalThis.__shareFallback.prompts.push({ message, value });
        return null;
      };
    });
    await page.route("**/api/v1/results", async (route) => {
      saveRequests += 1;
      await new Promise((resolve) => setTimeout(resolve, 100));
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ id: "ABCD1234" }),
      });
    });
    await page.goto("/");
    await page.evaluate(async () => {
      const { state } = await import("/state.js");
      state.phase = "results";
      state.runGeneration = 1;
      state.downloadResult = 100;
      state.uploadResult = 20;
      document.getElementById("idleState").classList.add("hidden");
      document.getElementById("resultsState").classList.remove("hidden");
      document.getElementById("shareBtn").classList.remove("hidden");
    });

    expect(saveRequests).toBe(0);
    await page.locator("#shareBtn").click();
    await expect.poll(() => saveRequests).toBe(1);
    await expect
      .poll(() => page.evaluate(() => globalThis.__shareFallback))
      .toEqual({
        clipboard: 1,
        share: 1,
        prompts: [
          {
            message: "Copy this link:",
            value: new URL("/results/ABCD1234", baseURL).href,
          },
        ],
      });
  });

  test("ignores a stale share response from a previous run", async ({ page }) => {
    let saveRequests = 0;
    let releaseFirst;
    const firstGate = new Promise((resolve) => {
      releaseFirst = resolve;
    });
    await page.route("**/api/v1/results", async (route) => {
      const requestNumber = ++saveRequests;
      if (requestNumber === 1) await firstGate;
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          id: requestNumber === 1 ? "OLDID123" : "NEWID123",
        }),
      });
    });
    await page.goto("/");

    await page.evaluate(async () => {
      const { state } = await import("/state.js");
      const { saveAndEnableShare } = await import(
        "/openbyte.js"
      );
      state.phase = "results";
      state.runGeneration = 10;
      state.shareSavePromise = null;
      globalThis.__oldShareSave = saveAndEnableShare();
    });
    await expect.poll(() => saveRequests).toBe(1);
    await page.evaluate(async () => {
      const { state } = await import("/state.js");
      const { saveAndEnableShare } = await import(
        "/openbyte.js"
      );
      state.runGeneration = 11;
      state.resultId = null;
      state.shareSavePromise = null;
      globalThis.__newShareSave = saveAndEnableShare();
    });
    await expect.poll(() => saveRequests).toBe(2);
    await page.evaluate(() => globalThis.__newShareSave);

    releaseFirst();
    const finalState = await page.evaluate(async () => {
      await globalThis.__oldShareSave;
      const { state } = await import("/state.js");
      return {
        resultId: state.resultId,
        saving: state.shareSavePromise !== null,
      };
    });
    expect(finalState).toEqual({ resultId: "NEWID123", saving: false });
  });

  test("a delayed aborted run cannot cancel its replacement", async ({
    page,
  }) => {
    await page.addInitScript(() => {
      const originalFetch = globalThis.fetch.bind(globalThis);
      let heldFirstMeasurement = false;
      globalThis.__delayedAbort = { entered: false, released: false };
      globalThis.fetch = (input, init) => {
        const url = typeof input === "string" ? input : input.url;
        const isMeasurementPing =
          url.includes("/api/v1/ping") &&
          init?.method === "GET" &&
          init?.cache !== "no-store";
        if (!heldFirstMeasurement && isMeasurementPing) {
          heldFirstMeasurement = true;
          globalThis.__delayedAbort.entered = true;
          return new Promise((_, reject) => {
            const rejectLater = () => {
              setTimeout(() => {
                globalThis.__delayedAbort.released = true;
                reject(new DOMException("Aborted", "AbortError"));
              }, 700);
            };
            if (init.signal.aborted) rejectLater();
            else {
              init.signal.addEventListener("abort", rejectLater, { once: true });
            }
          });
        }
        return originalFetch(input, init);
      };
    });
    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
    await page.locator("#startBtn").click();
    await expect
      .poll(() => page.evaluate(() => globalThis.__delayedAbort.entered))
      .toBe(true);

    await page.evaluate(() => {
      document.getElementById("cancelBtn").click();
      document.getElementById("startBtn").click();
    });
    await expect
      .poll(() => page.evaluate(() => globalThis.__delayedAbort.released))
      .toBe(true);
    const restartedRun = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      return {
        phase: state.phase,
        isRunning: state.isRunning,
        aborted: state.abortController?.signal.aborted,
      };
    });
    expect(restartedRun.phase).not.toBe("idle");
    expect(restartedRun.isRunning).toBe(true);
    expect(restartedRun.aborted).toBe(false);
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
  });

  test("light-theme labels and controls meet contrast and target sizes", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 720 });
    await page.goto("/");
    const audit = await page.evaluate(() => {
      const rgb = (value) => value.match(/[\d.]+/g).slice(0, 3).map(Number);
      const luminance = (value) => {
        const channels = rgb(value).map((channel) => {
          const normalized = channel / 255;
          return normalized <= 0.04045
            ? normalized / 12.92
            : ((normalized + 0.055) / 1.055) ** 2.4;
        });
        return (
          0.2126 * channels[0] +
          0.7152 * channels[1] +
          0.0722 * channels[2]
        );
      };
      const contrast = (foreground, background) => {
        const values = [luminance(foreground), luminance(background)].sort(
          (a, b) => b - a,
        );
        return (values[0] + 0.05) / (values[1] + 0.05);
      };

      document.documentElement.dataset.theme = "light";
      document.getElementById("idleState").classList.add("hidden");
      document.getElementById("resultsState").classList.remove("hidden");
      const badge = document.getElementById("bufferbloatResult");
      badge.classList.add("bb-good");
      const background = getComputedStyle(document.body).backgroundColor;
      const themeRect = document
        .getElementById("themeToggle")
        .getBoundingClientRect();
      const summaryRect = document
        .querySelector(".stats-help summary")
        .getBoundingClientRect();
      return {
        badgeContrast: contrast(getComputedStyle(badge).color, background),
        summaryHeight: summaryRect.height,
        themeHeight: themeRect.height,
        themeWidth: themeRect.width,
      };
    });

    expect(audit.badgeContrast).toBeGreaterThanOrEqual(4.5);
    expect(audit.summaryHeight).toBeGreaterThanOrEqual(44);
    expect(audit.themeHeight).toBeGreaterThanOrEqual(44);
    expect(audit.themeWidth).toBeGreaterThanOrEqual(44);
  });
});
