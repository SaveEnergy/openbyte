const { test, expect } = require("@playwright/test");

function channelLuminance(channel) {
  const value = channel / 255;
  return value <= 0.04045
    ? value / 12.92
    : ((value + 0.055) / 1.055) ** 2.4;
}

function luminance(hex) {
  const channels = hex
    .replace("#", "")
    .match(/.{2}/g)
    .map((value) => Number.parseInt(value, 16));
  return (
    0.2126 * channelLuminance(channels[0]) +
    0.7152 * channelLuminance(channels[1]) +
    0.0722 * channelLuminance(channels[2])
  );
}

function contrast(first, second) {
  const values = [luminance(first), luminance(second)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

async function showSyntheticResult(page) {
  await page.evaluate(async () => {
    const { state } = await import("/state.js");
    const { showResults } = await import("/ui.js");
    Object.assign(state, {
      phase: "results",
      downloadResult: 320,
      uploadResult: 48,
      latencyResult: 10,
      jitterResult: 1.5,
      downloadLatency: 82,
      uploadLatency: 74,
    });
    showResults();
  });
}

test.describe("brand and localized layout", () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ colorScheme: "light" });
    await page.addInitScript(() => {
      localStorage.setItem("openbyte-language", "de");
    });
  });

  test("keeps mint branding while using accessible light-theme text", async ({
    page,
  }) => {
    await page.goto("/");
    const styles = await page.evaluate(() => {
      const root = getComputedStyle(document.documentElement);
      const read = (selector) => getComputedStyle(document.querySelector(selector));
      return {
        brand: root.getPropertyValue("--brand-primary").trim(),
        accent: root.getPropertyValue("--accent-primary").trim(),
        background: root.getPropertyValue("--bg-secondary").trim(),
        controlBorder: root.getPropertyValue("--control-border").trim(),
        buttonFont: read("#startBtn").fontFamily,
        hintFont: read(".start-btn-hint").fontFamily,
        selectFont: read("#languageSelect").fontFamily,
        displayFont: read(".start-btn-text").fontFamily,
        displayWeight: read(".start-btn-text").fontWeight,
        logoWeight: read(".logo").fontWeight,
        logoAccent: read(".logo-accent").boxShadow,
        instrumentRing: read("#startBtn").boxShadow,
        shareBorder: read("#shareBtn").borderTopColor,
        fontSynthesis: root.fontSynthesis,
      };
    });

    await page.emulateMedia({ colorScheme: "dark" });
    const dark = await page.evaluate(() => {
      const root = getComputedStyle(document.documentElement);
      return {
        background: root.getPropertyValue("--bg-secondary").trim(),
        controlBorder: root.getPropertyValue("--control-border").trim(),
      };
    });

    expect(styles.brand).toBe("#00d4aa");
    expect(styles.accent).toBe("#00796b");
    expect(contrast(styles.controlBorder, styles.background)).toBeGreaterThanOrEqual(
      3,
    );
    expect(
      contrast(dark.controlBorder, dark.background),
    ).toBeGreaterThanOrEqual(3);
    expect(styles.buttonFont).toContain("DM Sans");
    expect(styles.hintFont).toContain("DM Sans");
    expect(styles.selectFont).toContain("DM Sans");
    expect(styles.displayFont).toContain("JetBrains Mono");
    expect(styles.displayWeight).toBe("600");
    expect(styles.logoWeight).toBe("600");
    expect(styles.logoAccent).toContain("rgb(0, 212, 170)");
    expect(styles.instrumentRing).toContain("rgb(0, 212, 170)");
    expect(styles.instrumentRing).toContain("3px");
    expect(styles.shareBorder).toBe("rgb(0, 121, 107)");
    expect(contrast(styles.accent, styles.background)).toBeGreaterThanOrEqual(3);
    expect(styles.fontSynthesis).toBe("none");
  });

  test("keeps the preference capsule inside narrow and intermediate headers", async ({
    page,
  }) => {
    await page.goto("/");
    for (const width of [320, 390, 430, 600]) {
      await page.setViewportSize({ width, height: 800 });
      const layout = await page.evaluate(() => {
        const rect = (selector) => {
          const bounds = document.querySelector(selector).getBoundingClientRect();
          return { left: bounds.left, right: bounds.right, height: bounds.height };
        };
        return {
          scrollWidth: document.documentElement.scrollWidth,
          viewportWidth: innerWidth,
          capsule: rect(".preference-controls"),
          select: rect("#languageSelect"),
          serverStatus: getComputedStyle(
            document.getElementById("serverInfo"),
          ).display,
        };
      });
      expect(layout.scrollWidth).toBeLessThanOrEqual(layout.viewportWidth);
      expect(layout.capsule.left).toBeGreaterThanOrEqual(0);
      expect(layout.capsule.right).toBeLessThanOrEqual(layout.viewportWidth);
      expect(layout.select.height).toBeGreaterThanOrEqual(44);
      expect(layout.serverStatus).toBe("none");
    }
  });

  test("puts German result numbers before the loaded-latency advisory", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 800 });
    await page.goto("/");
    await showSyntheticResult(page);

    const layout = await page.evaluate(() => {
      const bounds = (selector) =>
        document.querySelector(selector).getBoundingClientRect();
      const cards = [...document.querySelectorAll(".result-card")].map((card) =>
        card.getBoundingClientRect(),
      );
      return {
        scrollWidth: document.documentElement.scrollWidth,
        firstCardTop: cards[0].top,
        secondCardTop: cards[1].top,
        cardsBottom: Math.max(cards[0].bottom, cards[1].bottom),
        metricsTop: bounds(".results-extra").top,
        metricsBottom: bounds(".results-extra").bottom,
        advisoryTop: bounds("#resultsAdvisory").top,
        extraLabelTransform: getComputedStyle(
          document.querySelector(".extra-label"),
        ).textTransform,
      };
    });

    expect(layout.scrollWidth).toBeLessThanOrEqual(320);
    expect(Math.abs(layout.firstCardTop - layout.secondCardTop)).toBeLessThan(2);
    expect(layout.metricsTop).toBeGreaterThanOrEqual(layout.cardsBottom);
    expect(layout.advisoryTop).toBeGreaterThanOrEqual(layout.metricsBottom);
    expect(layout.extraLabelTransform).toBe("none");
    await expect(page.locator("#resultsAdvisory")).toHaveText(
      "Die Latenz steigt unter Last. Anrufe und Spiele können dann stocken.",
    );
  });

  test("reserves 404 for missing shared results", async ({ page }) => {
    await page.route("**/api/v1/results/*", async (route) => {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: "internal error" }),
      });
    });
    await page.goto("/results/ABCDEF12");
    await expect(page.locator("h1")).toHaveCount(1);
    await expect(page.locator("h1")).toHaveText("Testergebnis");
    await expect(page.locator("#errorCode")).toBeHidden();

    await page.unrouteAll({ behavior: "wait" });
    await page.goto("/results/00000000");
    await expect(page.locator("#errorCode")).toBeVisible();
    await expect(page.locator("#errorCode")).toHaveText("404");
  });
});
