const { test, expect } = require("@playwright/test");

test.describe("preferences disclosure", () => {
  test("groups language, appearance, and result storage on every page", async ({
    page,
  }) => {
    for (const path of ["/", "/privacy", "/results/00000000"]) {
      await page.goto(path);

      const menu = page.locator("#preferencesMenu");
      await expect(menu).not.toHaveAttribute("open", "");
      await page.locator(".preferences-trigger").click();
      await expect(page.locator(".preferences-panel")).toBeVisible();
      await expect(page.locator("#languageSelect")).toBeVisible();
      await expect(page.locator('input[name="themeMode"]')).toHaveCount(3);
      await expect(page.locator("#historyPreference")).toBeVisible();
    }
  });

  test("supports keyboard dismissal and closes on outside interaction", async ({
    page,
  }) => {
    await page.goto("/");

    const menu = page.locator("#preferencesMenu");
    const trigger = page.locator(".preferences-trigger");
    await trigger.focus();
    await page.keyboard.press("Enter");
    await expect(menu).toHaveAttribute("open", "");

    await page.keyboard.press("Tab");
    await expect(page.locator("#languageSelect")).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(menu).not.toHaveAttribute("open", "");
    await expect(trigger).toBeFocused();

    await trigger.click();
    await expect(menu).toHaveAttribute("open", "");
    await page.locator(".logo").click();
    await expect(menu).not.toHaveAttribute("open", "");
  });

  test("never creates result storage before explicit opt-in", async ({
    page,
  }) => {
    await page.addInitScript(() => {
      localStorage.setItem(
        "openbyte-history",
        JSON.stringify([{ ts: Date.now(), down: 10, up: 5 }]),
      );
    });
    await page.goto("/privacy");

    expect(
      await page.evaluate(() => ({
        enabled: localStorage.getItem("openbyte-history-enabled"),
        entries: localStorage.getItem("openbyte-history"),
      })),
    ).toEqual({ enabled: null, entries: null });

    await page.locator(".preferences-trigger").click();
    const history = page.locator("#historyPreference");
    await expect(history).not.toBeChecked();
    await history.check();
    expect(
      await page.evaluate(() => ({
        enabled: localStorage.getItem("openbyte-history-enabled"),
        entries: localStorage.getItem("openbyte-history"),
      })),
    ).toEqual({ enabled: "true", entries: null });

    await history.uncheck();
    expect(
      await page.evaluate(() => ({
        enabled: localStorage.getItem("openbyte-history-enabled"),
        entries: localStorage.getItem("openbyte-history"),
      })),
    ).toEqual({ enabled: null, entries: null });
  });
});
