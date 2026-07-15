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
      };
    });

    expect(audit).toEqual({
      sameKeys: true,
      blankEnglish: [],
      blankGerman: [],
      placeholderMismatches: [],
      missingHTMLKeys: [],
    });
  });

  test("query locale wins, manual choice persists, and Auto restores browser locale", async ({
    browser,
  }) => {
    const context = await browser.newContext({ locale: "de-DE" });
    const page = await context.newPage();
    await page.addInitScript(() => {
      localStorage.setItem("openbyte-language", "en");
    });

    await page.goto("/?lang=de&maxStreams=2&measureDuration=1");
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#languageSelect")).toHaveValue("de");

    await page.locator("#languageSelect").focus();
    await page.locator("#languageSelect").selectOption("en");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.locator("#languageSelect")).toBeFocused();
    expect(new URL(page.url()).searchParams.get("lang")).toBeNull();
    expect(new URL(page.url()).searchParams.get("maxStreams")).toBe("2");
    expect(new URL(page.url()).searchParams.get("measureDuration")).toBe("1");
    expect(
      await page.evaluate(() => localStorage.getItem("openbyte-language")),
    ).toBe("en");

    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await page.locator("#languageSelect").selectOption("auto");
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator('#languageSelect option[value="auto"]')).toHaveText(
      "System · DE",
    );
    expect(
      await page.evaluate(() => localStorage.getItem("openbyte-language")),
    ).toBeNull();
    await context.close();
  });

  test("System names the browser locale behind an explicit override", async ({
    page,
  }) => {
    await page.goto("/?lang=de");
    await expect(page.locator('#languageSelect option[value="auto"]')).toHaveText(
      "System · EN",
    );
    await page.locator("#languageSelect").selectOption("auto");
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
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
      const { formatLatency, formatSpeed, formatConnectionVerdict } =
        await import("/presentation.js");
      return {
        mbps: formatSpeed(123.45),
        gbps: formatSpeed(1234.56),
        latency: formatLatency(12.34),
        verdict: formatConnectionVerdict({
          download: 600,
          upload: 120,
          idleLatency: 10,
          loadedLatency: 12,
        }),
      };
    });
    expect(formatted.mbps).toEqual({ value: "123,5", unit: "Mbps" });
    expect(formatted.gbps).toEqual({ value: "1,23", unit: "Gbps" });
    expect(formatted.latency).toBe("12,3 ms");
    expect(formatted.verdict).toContain("Außergewöhnlich schnelle Verbindung");
  });

  test("localizes a shared result and preserves explicit locale links", async ({
    page,
    request,
  }) => {
    const payload = await createResult(request);
    await page.goto(`/results/${payload.id}?lang=de`);

    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#resultView")).toBeVisible();
    await expect(page.locator("#downloadResult")).toHaveText("123,5");
    await expect(page.locator("#uploadResult")).toHaveText("67,9");
    await expect(page.locator("#latencyResult")).toHaveText("12,3 ms");
    await expect(
      page.getByText("Öffentliche IP-Adressen beim Test"),
    ).toBeVisible();
    await expect(page.locator("#testedAt")).toContainText(/\d{1,2}\.\d{1,2}\.\d{4}/);
    await expect(page.locator("#resultsVerdict")).toContainText(
      "Ideal für Streaming und Videoanrufe.",
    );
    await expect(page.locator('a[data-locale-link]').first()).toHaveAttribute(
      "href",
      /[?&]lang=de(?:&|$)/,
    );
  });

  test("switches an active test live without cancelling its run", async ({
    page,
  }) => {
    await page.goto(
      "/?lang=en&maxStreams=1&measureDuration=1&rampDuration=1",
    );
    await page.locator("#startBtn").click();
    await expect
      .poll(() =>
        page.evaluate(async () => {
          const { state } = await import("/state.js");
          return state.phase;
        }),
      )
      .toMatch(/download|upload/);

    const before = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      return { generation: state.runGeneration, phase: state.phase };
    });
    await expect(page.locator("#phaseValuePing")).toHaveText(/\d+\.\d ms/);
    await page.locator("#languageSelect").selectOption("de");
    const after = await page.evaluate(async () => {
      const { state } = await import("/state.js");
      return {
        generation: state.runGeneration,
        phase: state.phase,
        aborted: state.abortController?.signal.aborted || false,
      };
    });

    expect(after.generation).toBe(before.generation);
    expect(after.phase).not.toBe("idle");
    expect(after.aborted).toBe(false);
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page.locator("#phaseValuePing")).toHaveText(/\d+,\d ms/);
    await expect(page.locator("#cancelBtn")).toHaveText("Abbrechen");
    await expect(page.locator("#resultsState")).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.locator("#restartBtn")).toHaveText("Nochmal testen");
    await expect(page.locator("#resultsVerdict")).not.toBeEmpty();
  });

  test("maps worker error codes without exposing raw English prose", async ({
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
            code: "download.network",
            message: "RAW ENGLISH WORKER DETAIL"
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
    await expect(page.locator("#errorMessage")).not.toContainText(
      "RAW ENGLISH",
    );
  });

  test("German controls and toast stay inside a 320px viewport", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 720 });
    await page.goto("/");
    await page.evaluate(async () => {
      const { showError } = await import("/ui.js");
      showError("error.serverOverloaded");
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
