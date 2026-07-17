const { test, expect } = require("@playwright/test");

test.describe("Privacy page", () => {
  test("serves the privacy page at the clean /privacy path", async ({
    page,
  }) => {
    await page.goto("/privacy");
    await expect(page).toHaveTitle("openByte — Privacy");
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Privacy and data handling",
      }),
    ).toBeVisible();
    await expect(page.locator(".legal-section")).toHaveCount(10);
    await expect(
      page.getByText(/not a complete operator-specific notice/),
    ).toBeVisible();
  });

  test("footer links the privacy page from the speed test", async ({
    page,
  }) => {
    await page.goto("/");
    const privacyLink = page.locator('.footer-links a[href="/privacy"]');
    await expect(privacyLink).toBeVisible();
    await privacyLink.click();
    await expect(page).toHaveURL(/\/privacy$/);
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Privacy and data handling",
      }),
    ).toBeVisible();
  });
});

test.describe("Privacy page in German", () => {
  test.use({ locale: "de-DE" });

  test("localizes the privacy page content and title", async ({ page }) => {
    await page.goto("/privacy");
    await expect(page.locator("html")).toHaveAttribute("lang", "de");
    await expect(page).toHaveTitle("openByte — Datenschutz");
    await expect(
      page.getByRole("heading", {
        level: 1,
        name: "Datenschutz und Datenverarbeitung",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Ergebnis teilen" }),
    ).toBeVisible();
    await expect(page.locator('.footer-links a[href="/privacy"]')).toHaveText(
      "Datenschutz",
    );
  });

  test("configured legal links wrap on a narrow German viewport", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 320, height: 800 });
    await page.route("**/branding.css", async (route) => {
      await route.fulfill({
        contentType: "text/css",
        body: ".footer-impressum { display: contents; }",
      });
    });
    await page.goto("/");

    await expect(page.locator('a[href="/impressum"]')).toBeVisible();
    const overflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth -
        document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
  });
});

test.describe("Impressum on unconfigured deployments", () => {
  test("keeps the footer link hidden and the route a 404 without IMPRESSUM_URL", async ({
    page,
    request,
  }) => {
    await page.goto("/");
    await expect(
      page.locator('.footer-links a[href="/impressum"]'),
    ).toBeHidden();

    const response = await request.get("/impressum", {
      maxRedirects: 0,
    });
    expect(response.status()).toBe(404);
  });
});
