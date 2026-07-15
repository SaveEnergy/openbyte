const { test, expect } = require("@playwright/test");

const resultPayload = {
  download_mbps: 123.45,
  upload_mbps: 67.89,
  latency_ms: 12.3,
  jitter_ms: 1.2,
  loaded_latency_ms: 18.4,
  bufferbloat_grade: "A",
  ipv4: "192.0.2.1",
  ipv6: "2001:db8::1",
  server_name: "playwright-server",
};

async function createResult(request, payload = resultPayload) {
  const response = await request.post("/api/v1/results", { data: payload });
  expect(response.ok()).toBeTruthy();
  return response.json();
}

test.describe("English and German localization", () => {
  test("catalogs have matching keys, placeholders, and HTML references", async ({
    page,
  }) => {
    await page.goto("/");
    const audit = await page.evaluate(async () => {
      const [{ en }, { de }] = await Promise.all([
        import("/locale-en.js"),
        import("/locale-de.js"),
      ]);
      const placeholders = (value) =>
        [...value.matchAll(/\{([a-zA-Z][a-zA-Z0-9]*)\}/g)]
          .map((match) => match[1])
          .sort();
      const enKeys = Object.keys(en).sort();
      const deKeys = Object.keys(de).sort();
      const semanticErrorKeys = [
        "worker.unsupported",
        "worker.failed",
        "worker.unreadable",
        "download.network",
        "upload.network",
        "server.overloaded",
        "download.noStreams",
        "upload.noStreams",
      ];
      const placeholderMismatches = enKeys.filter(
        (key) =>
          JSON.stringify(placeholders(en[key])) !==
          JSON.stringify(placeholders(de[key])),
      );
      const htmlKeys = new Set();
      for (const path of ["/index.html", "/results.html"]) {
        const html = await (await fetch(path)).text();
        const document = new DOMParser().parseFromString(html, "text/html");
        for (const element of document.querySelectorAll("*")) {
          for (const attribute of element.attributes) {
            if (attribute.name.startsWith("data-i18n")) {
              htmlKeys.add(attribute.value);
            }
          }
        }
      }
      return {
        sameKeys: JSON.stringify(enKeys) === JSON.stringify(deKeys),
        blankEnglish: enKeys.filter((key) => !en[key].trim()),
        blankGerman: deKeys.filter((key) => !de[key].trim()),
        placeholderMismatches,
        missingHTMLKeys: [...htmlKeys].filter((key) => !(key in en)),
        missingSemanticErrorKeys: semanticErrorKeys.filter(
          (key) => !(key in en),
        ),
      };
    });

    expect(audit).toEqual({
      sameKeys: true,
      blankEnglish: [],
      blankGerman: [],
      placeholderMismatches: [],
      missingHTMLKeys: [],
      missingSemanticErrorKeys: [],
    });
  });

  test("stored choice wins, reloads on change, and Auto restores browser locale", async ({
    browser,
  }) => {
    const context = await browser.newContext({ locale: "de-DE" });
    const page = await context.newPage();
    await page.goto("/");
    await page.evaluate(() => {
      localStorage.setItem("openbyte-language", "en");
    });

    await page.goto("/?lang=de&maxStreams=2&measureDuration=1");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.locator("#languageSelect")).toHaveValue("en");

    await Promise.all([
      page.waitForEvent("domcontentloaded"),
      page.locator("#languageSelect").selectOption("de"),
    ]);
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#languageSelect")).toHaveValue("de");
    expect(
      await page.evaluate(
        () => performance.getEntriesByType("navigation")[0]?.type,
      ),
    ).toBe("reload");
    expect(new URL(page.url()).searchParams.get("maxStreams")).toBe("2");
    expect(new URL(page.url()).searchParams.get("measureDuration")).toBe("1");
    expect(
      await page.evaluate(() => localStorage.getItem("openbyte-language")),
    ).toBe("de");

    await page.evaluate(() => {
      const url = new URL(globalThis.location.href);
      url.searchParams.set("lang", "en");
      history.replaceState(history.state, "", url);
    });
    await Promise.all([
      page.waitForEvent("domcontentloaded"),
      page.locator("#languageSelect").selectOption("auto"),
    ]);
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#languageSelect")).toHaveValue("auto");
    await expect(page.locator('#languageSelect option[value="auto"]')).toHaveText(
      "System · DE",
    );
    expect(
      await page.evaluate(() => localStorage.getItem("openbyte-language")),
    ).toBeNull();
    await context.close();
  });

  test("System names the browser locale behind a stored override", async ({
    page,
  }) => {
    await page.goto("/");
    await page.evaluate(() => {
      localStorage.setItem("openbyte-language", "de");
    });
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#languageSelect")).toHaveValue("de");
    await expect(page.locator('#languageSelect option[value="auto"]')).toHaveText(
      "System · EN",
    );
    await Promise.all([
      page.waitForEvent("domcontentloaded"),
      page.locator("#languageSelect").selectOption("auto"),
    ]);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.locator("#languageSelect")).toHaveValue("auto");
  });
});

test.describe("German UI", () => {
  test.use({ locale: "de-DE", timezoneId: "Europe/Berlin" });

  test("auto-detects German and localizes static, accessible, and formatted content", async ({
    page,
  }) => {
    await page.goto("/");

    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#languageSelect")).toHaveValue("auto");
    await expect(page.locator("#startBtn .start-btn-text")).toHaveText("GO");
    await expect(page.locator("#startBtn")).toHaveAttribute(
      "aria-label",
      /GO.*Speedtest starten/,
    );
    await expect(
      page.getByRole("heading", { name: "Öffentliche IP-Adressen" }),
    ).toBeVisible();
    await expect(page.locator(".stats-help summary")).toHaveText(
      "Was bedeuten die Werte?",
    );
    await expect(page.locator("#themeToggle")).toHaveAttribute(
      "aria-label",
      /Farbschema/,
    );
    await expect(page.locator('meta[name="description"]')).toHaveAttribute(
      "content",
      /Open-source internet speed test/,
    );

    const formatted = await page.evaluate(async () => {
      const { formatLatency, formatLoadedLatencyAdvisory, formatSpeed } =
        await import("/presentation.js");
      return {
        mbps: formatSpeed(123.45),
        gbps: formatSpeed(1234.56),
        latency: formatLatency(12.34),
        advisory: formatLoadedLatencyAdvisory({
          idleLatency: 10,
          loadedLatency: 82,
        }),
      };
    });
    expect(formatted.mbps).toEqual({ value: "123,5", unit: "Mbps" });
    expect(formatted.gbps).toEqual({ value: "1,23", unit: "Gbps" });
    expect(formatted.latency).toBe("12,3 ms");
    expect(formatted.advisory).toBe(
      "Die Latenz steigt unter Last. Anrufe und Spiele können dann stocken.",
    );
  });

  test("localizes a shared result without propagating locale links", async ({
    page,
    request,
  }) => {
    const payload = await createResult(request);
    await page.goto(`/results/${payload.id}`);

    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#resultView")).toBeVisible();
    await expect(page.locator("#downloadResult")).toHaveText("123,5");
    await expect(page.locator("#uploadResult")).toHaveText("67,9");
    await expect(page.locator("#latencyResult")).toHaveText("12,3 ms");
    await expect(
      page.getByText("Öffentliche IP-Adressen beim Test"),
    ).toBeVisible();
    await expect(page.locator("#testedAt")).toContainText(/\d{1,2}\.\d{1,2}\.\d{4}/);
    expect(
      await page.locator('a[href^="/"]').evaluateAll((links) =>
        links.some((link) => new URL(link.href).searchParams.has("lang")),
      ),
    ).toBe(false);
  });

  test("uses worker error codes directly as catalog keys", async ({
    page,
  }) => {
    await page.route("**/speedtest-worker.js", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/javascript",
        body: `globalThis.addEventListener("message", () => {
          globalThis.postMessage({
            type: "error",
            name: "Error",
            code: "download.network"
          });
        }, { once: true });`,
      });
    });
    await page.goto("/?maxStreams=1&measureDuration=1&rampDuration=1");
    await page.locator("#startBtn").click();

    await expect(page.locator("#errorToast")).toBeVisible({ timeout: 20_000 });
    await expect(page.locator("#errorMessage")).toHaveText(
      "Netzwerkfehler während des Downloads. Bitte erneut versuchen.",
    );
  });

  test("German controls and toast stay inside a 320px viewport", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 720 });
    await page.goto("/");
    await page.evaluate(async () => {
      const { showError } = await import("/ui.js");
      showError("server.overloaded");
    });
    await expect(page.locator("#errorToast")).toBeVisible();

    const audit = await page.evaluate(() => {
      const bounds = (element) => {
        const rect = element.getBoundingClientRect();
        return {
          left: rect.left,
          right: rect.right,
          height: rect.height,
        };
      };
      return {
        viewportWidth: innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        language: bounds(document.getElementById("languageSelect")),
        theme: bounds(document.getElementById("themeToggle")),
        toast: bounds(document.getElementById("errorToast")),
      };
    });

    expect(audit.scrollWidth).toBeLessThanOrEqual(audit.viewportWidth);
    for (const control of [audit.language, audit.theme, audit.toast]) {
      expect(control.left).toBeGreaterThanOrEqual(0);
      expect(control.right).toBeLessThanOrEqual(audit.viewportWidth);
    }
    expect(audit.language.height).toBeGreaterThanOrEqual(44);
    expect(audit.theme.height).toBeGreaterThanOrEqual(44);
  });
});
