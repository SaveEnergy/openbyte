const { test, expect } = require("@playwright/test");

test.describe("openByte UI", () => {
  test("loads and shows connected state", async ({ page }) => {
    await page.goto("/");
    const serverInfo = page.locator("#serverInfo");
    await expect(serverInfo).toContainText(
      /Connecting|OpenByte Server|Ready|Offline|Finding fastest/i,
    );
  });

  test("renders configured server name", async ({ page }) => {
    await page.route(/\/api\/v1\/ping\?meta=1$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          pong: true,
          client_ip: "198.51.100.10",
          ipv6: false,
          server_name: "Frankfurt 10G",
        }),
      });
    });

    await page.goto("/");

    await expect(page.locator("#serverName")).toHaveText("Frankfurt 10G");
  });

  test("runs a short adaptive test flow", async ({ page }) => {
    await page.addInitScript(() => {
      const OriginalWorker = globalThis.Worker;
      globalThis.__openbyteWorkerUrls = [];
      globalThis.Worker = class OpenByteTestWorker extends OriginalWorker {
        constructor(url, options) {
          globalThis.__openbyteWorkerUrls.push(String(url));
          super(url, options);
        }
      };
    });

    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");

    await expect(page.locator("#showSettings")).toHaveCount(0);
    await expect(page.locator("#duration")).toHaveCount(0);
    await expect(page.locator("#streams")).toHaveCount(0);

    await page.locator("#startBtn").click();
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
    const downloadMbps = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      return state.downloadResult;
    });
    expect(Number.isFinite(downloadMbps)).toBeTruthy();
    expect(downloadMbps).toBeGreaterThanOrEqual(0);
    const workerUrls = await page.evaluate(
      () => globalThis.__openbyteWorkerUrls,
    );
    expect(
      workerUrls.some((url) => url.includes("speedtest-worker.js")),
    ).toBeTruthy();
  });

  test("shows phase stepper during test with history after", async ({
    page,
  }) => {
    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");

    await page.locator("#startBtn").click();
    await expect(page.locator("#phaseSteps")).toBeVisible();
    await expect(page.locator("#phaseStepPing")).toHaveAttribute(
      "data-status",
      /active|done/,
    );

    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.locator("#historySection")).toBeVisible();
    await expect(page.locator("#historyList .history-item")).toHaveCount(1);
  });

  test("theme toggle cycles system, light, dark", async ({ page }) => {
    await page.goto("/");
    const html = page.locator("html");

    await expect(html).not.toHaveAttribute("data-theme", /.+/);
    await page.locator("#themeToggle").click();
    await expect(html).toHaveAttribute("data-theme", "light");
    await page.locator("#themeToggle").click();
    await expect(html).toHaveAttribute("data-theme", "dark");

    await page.reload();
    await expect(html).toHaveAttribute("data-theme", "dark");
    await page.locator("#themeToggle").click();
    await expect(html).not.toHaveAttribute("data-theme", /.+/);
  });

  test("toast regions keep accessible roles", async ({ page }) => {
    await page.goto("/");

    await expect(page.locator("#successToast")).toHaveJSProperty(
      "tagName",
      "OUTPUT",
    );
    await expect(page.locator("#successToast")).toHaveAttribute(
      "aria-live",
      "polite",
    );
    await expect(page.locator("#errorToast")).toHaveAttribute("role", "alert");
    await expect(page.locator("#errorToast")).toHaveAttribute(
      "aria-live",
      "assertive",
    );
    await expect(page.locator("#errorToast")).toBeHidden();
  });

  test("adaptive test focus follows state", async ({ page }) => {
    const pageErrors = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));

    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#startBtn")).toBeVisible();
    expect(pageErrors).toEqual([]);

    await page.locator("#startBtn").click();
    await expect(page.locator("#testingState")).toBeVisible({ timeout: 10000 });
    await expect(page.locator("#progressMeter")).toHaveAttribute(
      "value",
      /[\d.]+/,
    );
    await expect(page.locator("#testType")).toContainText(
      /Ping|Download|Upload/,
    );
    await expect(page.locator("#cancelBtn")).toBeFocused();

    await page.locator("#cancelBtn").click();
    await expect(page.locator("#idleState")).toBeVisible({ timeout: 10000 });
    await expect(page.locator("#startBtn")).toBeFocused();

    await page.locator("#startBtn").click();
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.locator("#restartBtn")).toBeFocused();
  });

  test("loaded latency probe aborts hung ping during completion", async ({
    page,
  }) => {
    const pageErrors = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));
    await page.addInitScript(() => {
      const originalFetch = globalThis.fetch.bind(globalThis);
      globalThis.__loadedLatencyProbe = { entered: false, aborted: false };
      globalThis.fetch = (input, init) => {
        const url = typeof input === "string" ? input : input.url;
        const probe = globalThis.__loadedLatencyProbe;
        const isLoadedLatency =
          url.includes("/api/v1/ping") &&
          init?.method === "GET" &&
          init?.cache === "no-store";
        if (!probe.entered && isLoadedLatency) {
          probe.entered = true;
          return new Promise((_, reject) => {
            const signal = init?.signal;
            if (!signal) {
              reject(new Error("loaded-latency request missing abort signal"));
              return;
            }
            const abort = () => {
              probe.aborted = true;
              signal.removeEventListener("abort", abort);
              reject(new DOMException("Aborted", "AbortError"));
            };
            if (signal.aborted) {
              abort();
              return;
            }
            signal.addEventListener("abort", abort, { once: true });
          });
        }
        return originalFetch(input, init);
      };
    });

    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");

    await page.locator("#startBtn").click();
    await expect
      .poll(
        () => page.evaluate(() => globalThis.__loadedLatencyProbe.entered),
        { timeout: 15_000 },
      )
      .toBe(true);
    await expect
      .poll(
        () => page.evaluate(() => globalThis.__loadedLatencyProbe.aborted),
        { timeout: 15_000 },
      )
      .toBe(true);
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 20_000,
    });
    expect(pageErrors).toEqual([]);
  });

  test("saves result only when share is clicked", async ({ page }) => {
    let saveRequests = 0;
    let savePayload = null;
    page.on("dialog", async (dialog) => {
      await dialog.dismiss().catch(() => {});
    });
    await page.route("**/api/v1/results", async (route) => {
      saveRequests += 1;
      savePayload = route.request().postDataJSON();
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ id: "ABCD1234", url: "/results/ABCD1234" }),
      });
    });

    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");

    await page.locator("#startBtn").click();
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.locator("#shareBtn")).toBeVisible();
    await expect.poll(() => saveRequests).toBe(0);

    await page.locator("#shareBtn").click();
    await expect.poll(() => saveRequests).toBe(1);
    expect(savePayload).not.toHaveProperty("diagnostics");
  });

  test("discards an upload-phase cancel then restarts cleanly", async ({
    page,
  }) => {
    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");

    await page.locator("#startBtn").click();
    await expect
      .poll(
        () =>
          page.evaluate(async () => {
            const { state } = await import("/state.js");
            return {
              phase: state.phase,
              downloadComplete: state.downloadResult > 0,
            };
          }),
        { timeout: 60_000 },
      )
      .toEqual({ phase: "upload", downloadComplete: true });

    await page.locator("#cancelBtn").click();
    await expect(page.locator("#idleState")).toBeVisible();
    await expect(page.locator("#startBtn")).toBeFocused();
    await expect(page.locator("#shareBtn")).toBeHidden();

    const cancelled = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      return {
        phase: state.phase,
        isRunning: state.isRunning,
        hasController: state.abortController !== null,
        download: state.downloadResult,
        upload: state.uploadResult,
        latency: state.latencyResult,
        jitter: state.jitterResult,
        downloadLatency: state.downloadLatency,
        uploadLatency: state.uploadLatency,
        history: JSON.parse(localStorage.getItem("openbyte-history") || "[]"),
      };
    });
    expect(cancelled).toEqual({
      phase: "idle",
      isRunning: false,
      hasController: false,
      download: 0,
      upload: 0,
      latency: null,
      jitter: null,
      downloadLatency: 0,
      uploadLatency: 0,
      history: [],
    });

    await page.locator("#startBtn").click();
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.locator("#loadedLatencyResult")).not.toHaveText("-");
    await expect(page.locator("#bufferbloatResult")).toHaveText(
      /^(A\+|A|B|C|D|F)$/,
    );
    await expect(page.locator(".stats-help")).toBeVisible();
    await expect(page.locator("#historyList .history-item")).toHaveCount(1);
    await expect(page.locator("#shareBtn")).toBeVisible();
  });

  test("renders shared result page from saved result", async ({
    page,
    request,
  }) => {
    const create = await request.post("/api/v1/results", {
      data: {
        download_mbps: 123.45,
        upload_mbps: 67.89,
        latency_ms: 12.3,
        jitter_ms: 1.2,
        loaded_latency_ms: 80.4,
        bufferbloat_grade: "D",
        ipv4: "192.0.2.1",
        ipv6: "2001:db8::1",
        server_name: "playwright-server",
      },
    });
    expect(create.ok()).toBeTruthy();
    const payload = await create.json();
    expect(payload.id).toMatch(/^[0-9a-zA-Z]{8}$/);

    await page.goto("/results/" + payload.id);

    await expect(page.locator("#resultView")).toBeVisible();
    await expect(page.locator("#downloadResult")).toContainText("123.5");
    await expect(page.locator("#uploadResult")).toContainText("67.9");
    await expect(page.locator("#loadedLatencyResult")).toHaveText("80.4 ms");
    await expect(page.locator("#bufferbloatResult")).toHaveText("D");
    await expect(page.locator("#resultsAdvisory")).toContainText(
      "Latency rises under load",
    );
    await expect(
      page.getByText("Public IP addresses for this test"),
    ).toBeVisible();
    await expect(page.locator("#networkIPv4")).toHaveText("192.0.2.1");
    await expect(page.locator("#networkIPv6")).toHaveText("2001:db8::1");
    await expect(page.locator("#serverValue")).toContainText(
      "playwright-server",
    );
    await expect(page.locator("#errorView")).toHaveClass(/hidden/);
  });

  test("renders Gbps unit for high shared result speeds", async ({
    page,
    request,
  }) => {
    const create = await request.post("/api/v1/results", {
      data: {
        download_mbps: 1234.56,
        upload_mbps: 1500.12,
        latency_ms: 8.1,
        jitter_ms: 0.8,
        loaded_latency_ms: 12.4,
        bufferbloat_grade: "A",
        ipv4: "192.0.2.1",
        ipv6: "",
        server_name: "gbps-server",
      },
    });
    expect(create.ok()).toBeTruthy();
    const payload = await create.json();
    expect(payload.id).toMatch(/^[0-9a-zA-Z]{8}$/);

    await page.goto("/results/" + payload.id);
    await expect(page.locator("#resultView")).toBeVisible();
    await expect(page.locator("#downloadResult")).toContainText("1.23");
    await expect(page.locator("#uploadResult")).toContainText("1.50");
    await expect(page.locator(".result-primary .result-unit")).toContainText(
      "Gbps",
    );
    await expect(page.locator(".result-secondary .result-unit")).toContainText(
      "Gbps",
    );
  });

  test("shows error view for invalid shared result id", async ({ page }) => {
    const response = await page.goto("/results/invalid-id");
    expect(response).toBeTruthy();
    expect(response.status()).toBe(404);
  });

  test("shows not-found message for missing shared result", async ({
    page,
  }) => {
    await page.goto("/results/00000000");
    await expect(page.locator("#errorView")).toBeVisible();
    await expect(page.locator("#resultView")).toHaveClass(/hidden/);
    await expect(page.locator("#errorView .error-message")).toContainText(
      /not found|expired/i,
    );
    await expect(page.locator("#errorCode")).toBeVisible();
  });

  test("shows server-error message when results API returns 500", async ({
    page,
  }) => {
    await page.route("**/api/v1/results/*", async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: "internal error" }),
      });
    });

    await page.goto("/results/ABCDEF12");
    await expect(page.locator("#errorView")).toBeVisible();
    await expect(page.locator("#resultView")).toHaveClass(/hidden/);
    await expect(page.locator("#errorView .error-message")).toContainText(
      /server error/i,
    );
    await expect(page.locator("#errorCode")).toBeHidden();
  });

  test("shows timeout message when results API hangs", async ({ page }) => {
    await page.clock.install();
    let markRequestStarted;
    const requestStarted = new Promise((resolve) => {
      markRequestStarted = resolve;
    });
    await page.route("**/api/v1/results/*", async () => {
      markRequestStarted();
      await new Promise(() => {});
    });

    await page.goto("/results/ABCDEF12", { waitUntil: "commit" });
    await requestStarted;
    await page.clock.fastForward(20_000);

    await expect(page.locator("#errorView")).toBeVisible();
    await expect(page.locator("#resultView")).toHaveClass(/hidden/);
    await expect(page.locator("#errorView .error-message")).toContainText(
      /timed out/i,
    );
  });

  test("shows fallback error message for malformed results payload", async ({
    page,
  }) => {
    await page.route("**/api/v1/results/*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: "{not-json",
      });
    });

    await page.goto("/results/ABCDEF12");
    await expect(page.locator("#errorView")).toBeVisible();
    await expect(page.locator("#resultView")).toHaveClass(/hidden/);
    await expect(page.locator("#errorView .error-message")).toContainText(
      /unable to load result/i,
    );
  });
});
