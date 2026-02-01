const { test, expect } = require('@playwright/test');

test.describe('openByte UI', () => {
  test('loads and shows connected state', async ({ page }) => {
    await page.goto('/');
    const serverInfo = page.locator('#serverInfo');
    await expect(serverInfo).toContainText(/Connecting|OpenByte Server|Ready/i);
  });

  test('runs a short test flow', async ({ page }) => {
    await page.goto('/');

    await page.locator('#showSettings').click();
    await page.selectOption('#duration', '5');
    await page.selectOption('#streams', '1');
    await page.locator('#closeSettings').click();

    await page.locator('#startBtn').click();
    await expect(page.locator('#resultsState')).toBeVisible({ timeout: 60_000 });
    await expect(page.locator('#downloadResult')).not.toHaveText('0');
  });
});
