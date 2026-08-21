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
      await expect(page.locator(".theme-option-icon")).toHaveCount(3);
      await expect(page.getByRole("radio", { name: "System" })).toBeVisible();
      await expect(page.getByRole("radio", { name: "Light" })).toBeVisible();
      await expect(page.getByRole("radio", { name: "Dark" })).toBeVisible();
      await expect(page.getByRole("switch")).toBeVisible();
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

    await trigger.click();
    await page.getByRole("switch").focus();
    await page.keyboard.press("Tab");
    await expect(menu).not.toHaveAttribute("open", "");
  });

  test("keeps theme tiles clickable when pointer input does not move focus", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator(".preferences-trigger").click();

    const menu = page.locator("#preferencesMenu");
    const light = page.getByRole("radio", { name: "Light" });
    const lightTile = page.locator('.theme-option:has(input[value="light"])');
    await lightTile.evaluate((tile) => {
      tile.addEventListener(
        "pointerdown",
        () => {
          if (document.activeElement instanceof HTMLElement) {
            document.activeElement.blur();
          }
        },
        { once: true },
      );
    });

    const bounds = await lightTile.boundingBox();
    expect(bounds).not.toBeNull();
    await page.mouse.move(
      bounds.x + bounds.width / 2,
      bounds.y + bounds.height / 2,
    );
    await page.mouse.down();
    await page.waitForTimeout(25);
    await expect(menu).toHaveAttribute("open", "");
    await page.mouse.up();
    await expect(light).toBeChecked();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  });

  test("keeps native keyboard behavior for theme tiles and history switch", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator(".preferences-trigger").click();

    const system = page.getByRole("radio", { name: "System" });
    const light = page.getByRole("radio", { name: "Light" });
    const history = page.getByRole("switch");
    await expect(system).toBeChecked();

    await system.focus();
    await page.keyboard.press("ArrowRight");
    await expect(light).toBeChecked();
    expect(
      await page.evaluate(() => localStorage.getItem("openbyte-theme")),
    ).toBe("light");

    await page.keyboard.press("Tab");
    await expect(history).toBeFocused();
    await page.keyboard.press("Space");
    await expect(history).toBeChecked();
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
    const history = page.getByRole("switch");
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

  test("removes the legacy automatic probe cache during upgrade", async ({
    page,
  }) => {
    await page.addInitScript(() => {
      localStorage.setItem("openbyte-probe-skip", '{"v4.example":4102444800000}');
    });
    await page.goto("/");

    expect(
      await page.evaluate(() => localStorage.getItem("openbyte-probe-skip")),
    ).toBeNull();
  });
});
