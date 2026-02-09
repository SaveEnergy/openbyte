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
    const downloadText = await page.locator('#downloadResult').textContent();
    const downloadMbps = parseFloat(downloadText || '0');
    expect(Number.isFinite(downloadMbps)).toBeTruthy();
    expect(downloadMbps).toBeGreaterThan(0);
  });

  test('handles cancel then restart cleanly', async ({ page }) => {
    await page.goto('/');

    await page.locator('#showSettings').click();
    await page.selectOption('#duration', '5');
    await page.selectOption('#streams', '1');
    await page.locator('#closeSettings').click();

    await page.locator('#startBtn').click();
    await page.waitForTimeout(700);
    await page.locator('#cancelBtn').click();
    await page.locator('#startBtn').click();

    await expect(page.locator('#resultsState')).toBeVisible({ timeout: 60_000 });
    const downloadText = await page.locator('#downloadResult').textContent();
    const downloadMbps = parseFloat(downloadText || '0');
    expect(Number.isFinite(downloadMbps)).toBeTruthy();
    expect(downloadMbps).toBeGreaterThan(0);
  });

  test('skill page uses external scripts only', async ({ page }) => {
    await page.goto('/skill.html');

    await expect(page.locator('h1')).toHaveText(/Agent Skill/i);
    await expect(page.locator('#copySkillBtn')).toBeVisible();

    const scripts = await page.locator('script').evaluateAll((nodes) =>
      nodes.map((n) => ({
        src: n.getAttribute('src'),
        inline: (n.textContent || '').trim().length > 0,
      }))
    );

    expect(scripts.length).toBeGreaterThan(0);
    for (const script of scripts) {
      expect(script.src).toBeTruthy();
      expect(script.inline).toBeFalsy();
    }
  });
});
