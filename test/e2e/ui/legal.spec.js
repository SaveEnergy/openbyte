const { test, expect } = require("@playwright/test");

test.describe("Privacy page", () => {
  test("serves the privacy page at the clean /privacy path", async ({
    page,
  }) => {
    await page.goto("/privacy");
    await expect(page).toHaveTitle("openByte — Privacy");
    await expect(
      page.getByRole("heading", { level: 1, name: "Data privacy" }),
    ).toBeVisible();
    await expect(page.locator(".legal-section")).toHaveCount(7);
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
      page.getByRole("heading", { level: 1, name: "Data privacy" }),
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
      page.getByRole("heading", { level: 1, name: "Datenschutz" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Geteilte Ergebnisse" }),
    ).toBeVisible();
    await expect(page.locator('.footer-links a[href="/privacy"]')).toHaveText(
      "Datenschutz",
    );
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
