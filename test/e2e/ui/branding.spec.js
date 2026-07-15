const { test, expect } = require("@playwright/test");

const brandingCSS = `
:root {
  --brand-primary: #ffcc00;
  --brand-secondary: #e0b400;
  --on-brand: #111111;
  --accent-primary: #ffcc00;
  --accent-secondary: #e0b400;
  --accent-glow: rgba(255, 204, 0, 0.3);
  --ambient-primary: rgba(255, 204, 0, 0.08);
  --ambient-secondary: rgba(79, 140, 255, 0.05);
  --download-color: #ffcc00;
  --upload-color: #4f8cff;
}

@media (prefers-color-scheme: light) {
  :root {
    --brand-primary: #805f00;
    --brand-secondary: #6b4f00;
    --on-brand: #ffffff;
    --accent-primary: #805f00;
    --accent-secondary: #6b4f00;
    --accent-glow: rgba(128, 95, 0, 0.22);
    --ambient-primary: rgba(128, 95, 0, 0.08);
    --ambient-secondary: rgba(49, 93, 184, 0.05);
    --download-color: #805f00;
    --upload-color: #315db8;
  }
}

.brand-wordmark { display: none; }
.brand-logo { display: block; }
`;

const logoSVG = `
<svg xmlns="http://www.w3.org/2000/svg" width="360" height="64" viewBox="0 0 360 64">
  <rect width="360" height="64" rx="8" fill="#ffcc00" />
  <text x="180" y="42" text-anchor="middle" font-family="sans-serif" font-size="28">ACME NET</text>
</svg>
`;

async function mockBranding(page, logoGate = Promise.resolve()) {
  await page.route("**/branding.css", (route) =>
    route.fulfill({ contentType: "text/css", body: brandingCSS }),
  );
  await page.route("**/branding/logo", async (route) => {
    await logoGate;
    await route.fulfill({ contentType: "image/svg+xml", body: logoSVG });
  });
}

test.describe("visual branding", () => {
  test("uses generated colors and reserves the mobile logo slot", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 720 });
    await page.emulateMedia({ colorScheme: "dark" });

    let releaseLogo;
    const logoGate = new Promise((resolve) => {
      releaseLogo = resolve;
    });
    let releaseMetadata;
    const metadataGate = new Promise((resolve) => {
      releaseMetadata = resolve;
    });
    await page.route("**/api/v1/ping?meta=1", async (route) => {
      await metadataGate;
      await route.continue();
    });
    await mockBranding(page, logoGate);
    await page.goto("/?lang=de", { waitUntil: "domcontentloaded" });

    const logo = page.locator(".brand-logo");
    await expect(page.locator(".brand-wordmark")).toBeHidden();
    await expect(logo).toBeVisible();
    const beforeLoad = await logo.boundingBox();

    releaseLogo();
    await expect
      .poll(() => logo.evaluate((image) => image.naturalWidth))
      .toBe(360);
    const afterLoad = await logo.boundingBox();

    expect(beforeLoad).not.toBeNull();
    expect(afterLoad).toEqual(beforeLoad);
    expect(afterLoad.width).toBeLessThanOrEqual(180);
    expect(afterLoad.height).toBe(32);

    const mobileServerName = await page.locator("#serverName").evaluate((node) => {
      const styles = getComputedStyle(node);
      return {
        display: styles.display,
        clipPath: styles.clipPath,
        text: node.textContent,
      };
    });
    expect(mobileServerName).toEqual({
      display: "block",
      clipPath: "inset(50%)",
      text: "openByte Server",
    });

    await page.setViewportSize({ width: 720, height: 720 });
    await expect(page.locator(".server-info")).toBeHidden();
    expect((await logo.boundingBox()).width).toBe(180);
    await page.setViewportSize({ width: 721, height: 720 });
    await expect(page.locator(".server-info")).toBeVisible();
    expect((await logo.boundingBox()).width).toBe(180);
    releaseMetadata();
    await page.setViewportSize({ width: 320, height: 720 });

    const audit = await page.evaluate(() => {
      const styles = getComputedStyle(document.documentElement);
      return {
        brand: styles.getPropertyValue("--brand-primary").trim(),
        accent: styles.getPropertyValue("--accent-primary").trim(),
        download: styles.getPropertyValue("--download-color").trim(),
        upload: styles.getPropertyValue("--upload-color").trim(),
        onBrand: styles.getPropertyValue("--on-brand").trim(),
        success: styles.getPropertyValue("--success").trim(),
        scrollWidth: document.documentElement.scrollWidth,
        viewportWidth: innerWidth,
      };
    });
    expect(audit).toEqual({
      brand: "#ffcc00",
      accent: "#ffcc00",
      download: "#ffcc00",
      upload: "#4f8cff",
      onBrand: "#111111",
      success: "#00d4aa",
      scrollWidth: 320,
      viewportWidth: 320,
    });

    const semanticColors = await page.evaluate(() => {
      const dot = document.querySelector(".server-dot");
      const badge = document.getElementById("bufferbloatResult");
      dot.classList.remove("error");
      dot.classList.add("connected");
      badge.classList.remove("bb-mid", "bb-bad");
      badge.classList.add("bb-good");
      return {
        connected: getComputedStyle(dot).backgroundColor,
        good: getComputedStyle(badge).color,
      };
    });
    expect(semanticColors).toEqual({
      connected: "rgb(0, 212, 170)",
      good: "rgb(0, 212, 170)",
    });

    await page.emulateMedia({ colorScheme: "light" });
    await expect
      .poll(() =>
        page.evaluate(() => {
          const styles = getComputedStyle(document.documentElement);
          return [
            styles.getPropertyValue("--brand-primary").trim(),
            styles.getPropertyValue("--accent-primary").trim(),
            styles.getPropertyValue("--download-color").trim(),
            styles.getPropertyValue("--upload-color").trim(),
          ];
        }),
      )
      .toEqual(["#805f00", "#805f00", "#805f00", "#315db8"]);

    await page.emulateMedia({ colorScheme: "dark" });
    await expect
      .poll(() =>
        page.evaluate(() => {
          const styles = getComputedStyle(document.documentElement);
          return [
            styles.getPropertyValue("--brand-primary").trim(),
            styles.getPropertyValue("--accent-primary").trim(),
            styles.getPropertyValue("--download-color").trim(),
            styles.getPropertyValue("--upload-color").trim(),
          ];
        }),
      )
      .toEqual(["#ffcc00", "#ffcc00", "#ffcc00", "#4f8cff"]);

    const instrumentColors = await page.evaluate(async () => {
      const { updateTestType } = await import("/ui.js");
      const testingState = document.getElementById("testingState");

      updateTestType("test.phase.download", "downloading");
      const download = getComputedStyle(testingState)
        .getPropertyValue("--instrument-color")
        .trim();

      updateTestType("test.phase.upload", "uploading");
      const upload = getComputedStyle(testingState)
        .getPropertyValue("--instrument-color")
        .trim();
      return { download, upload };
    });
    expect(instrumentColors).toEqual({
      download: "#ffcc00",
      upload: "#4f8cff",
    });
  });

  test("keeps the branded result-page home link accessible", async ({
    page,
  }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await mockBranding(page);
    await page.goto("/results/abc12345?lang=en");

    const home = page
      .locator("header")
      .getByRole("link", { name: "Speed Test", exact: true });
    await expect(home).toBeVisible();
    await expect(home.locator(".brand-logo")).toBeVisible();

    await expect
      .poll(() =>
        page.evaluate(() =>
          getComputedStyle(document.documentElement)
            .getPropertyValue("--brand-primary")
            .trim(),
        ),
      )
      .toBe("#ffcc00");
    await page.emulateMedia({ colorScheme: "light" });
    await expect
      .poll(() =>
        page.evaluate(() =>
          getComputedStyle(document.documentElement)
            .getPropertyValue("--brand-primary")
            .trim(),
        ),
      )
      .toBe("#805f00");
  });
});
