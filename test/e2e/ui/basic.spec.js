const { test, expect } = require('@playwright/test');

test.describe('openByte UI', () => {
  test('loads and shows connected state', async ({ page }) => {
    await page.goto('/');
    const serverInfo = page.locator('#serverInfo');
    await expect(serverInfo).toContainText(/Connecting|OpenByte Server|Ready|Offline|Finding fastest/i);
  });

  test('runs a short test flow', async ({ page }) => {
    await page.goto('/');

    await page.locator('#showSettings').click();
    await expect(page.locator('#settingsModal')).toBeVisible();
    await page.selectOption('#duration', '5');
    await page.selectOption('#streams', '1');
    await page.locator('#closeSettings').click();

    await page.locator('#startBtn').click();
    await expect(page.locator('#resultsState')).toBeVisible({ timeout: 60_000 });
    const downloadText = await page.locator('#downloadResult').textContent();
    const downloadMbps = parseFloat(downloadText || '0');
    expect(Number.isFinite(downloadMbps)).toBeTruthy();
    expect(downloadMbps).toBeGreaterThanOrEqual(0);
  });

  test('saves result only when share is clicked', async ({ page }) => {
    let saveRequests = 0;
    page.on('dialog', async (dialog) => {
      await dialog.dismiss().catch(() => {});
    });
    await page.route('**/api/v1/results', async (route) => {
      saveRequests += 1;
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'ABCD1234', url: '/results/ABCD1234' })
      });
    });

    await page.goto('/');
    await page.locator('#showSettings').click();
    await expect(page.locator('#settingsModal')).toBeVisible();
    await page.selectOption('#duration', '5');
    await page.selectOption('#streams', '1');
    await page.locator('#closeSettings').click();

    await page.locator('#startBtn').click();
    await expect(page.locator('#resultsState')).toBeVisible({ timeout: 60_000 });
    await expect(page.locator('#shareBtn')).toBeVisible();
    await expect.poll(() => saveRequests).toBe(0);

    await page.locator('#shareBtn').click();
    await expect.poll(() => saveRequests).toBe(1);
  });

  test('handles cancel then restart cleanly', async ({ page }) => {
    await page.goto('/');

    await page.locator('#showSettings').click();
    await expect(page.locator('#settingsModal')).toBeVisible();
    await page.selectOption('#duration', '5');
    await page.selectOption('#streams', '1');
    await page.locator('#closeSettings').click();

    await page.locator('#startBtn').click();
    await expect(page.locator('#testingState')).toBeVisible({ timeout: 10000 });
    await page.locator('#cancelBtn').click();
    await expect(page.locator('#idleState')).toBeVisible({ timeout: 10000 });
    await page.locator('#startBtn').click();

    await expect(page.locator('#resultsState')).toBeVisible({ timeout: 60_000 });
    const downloadText = await page.locator('#downloadResult').textContent();
    const downloadMbps = parseFloat(downloadText || '0');
    expect(Number.isFinite(downloadMbps)).toBeTruthy();
    expect(downloadMbps).toBeGreaterThanOrEqual(0);
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

  test('renders shared result page from saved result', async ({ page, request }) => {
    const create = await request.post('/api/v1/results', {
      data: {
        download_mbps: 123.45,
        upload_mbps: 67.89,
        latency_ms: 12.3,
        jitter_ms: 1.2,
        loaded_latency_ms: 18.4,
        bufferbloat_grade: 'A',
        ipv4: '192.0.2.1',
        ipv6: '',
        server_name: 'playwright-server'
      }
    });
    expect(create.ok()).toBeTruthy();
    const payload = await create.json();
    expect(payload.id).toMatch(/^[0-9a-zA-Z]{8}$/);

    await page.goto('/results/' + payload.id);

    await expect(page.locator('#resultView')).toBeVisible();
    await expect(page.locator('#downloadResult')).toContainText('123.5');
    await expect(page.locator('#uploadResult')).toContainText('67.9');
    await expect(page.locator('#serverValue')).toContainText('playwright-server');
    await expect(page.locator('#errorView')).toHaveClass(/hidden/);
  });

  test('renders Gbps unit for high shared result speeds', async ({ page, request }) => {
    const create = await request.post('/api/v1/results', {
      data: {
        download_mbps: 1234.56,
        upload_mbps: 1500.12,
        latency_ms: 8.1,
        jitter_ms: 0.8,
        loaded_latency_ms: 12.4,
        bufferbloat_grade: 'A',
        ipv4: '192.0.2.1',
        ipv6: '',
        server_name: 'gbps-server'
      }
    });
    expect(create.ok()).toBeTruthy();
    const payload = await create.json();
    expect(payload.id).toMatch(/^[0-9a-zA-Z]{8}$/);

    await page.goto('/results/' + payload.id);
    await expect(page.locator('#resultView')).toBeVisible();
    await expect(page.locator('#downloadResult')).toContainText('1.23');
    await expect(page.locator('#uploadResult')).toContainText('1.50');
    await expect(page.locator('.result-primary .result-unit')).toContainText('Gbps');
    await expect(page.locator('.result-secondary .result-unit')).toContainText('Gbps');
  });

  test('shows error view for invalid shared result id', async ({ page }) => {
    const response = await page.goto('/results/invalid-id');
    expect(response).toBeTruthy();
    expect(response.status()).toBe(404);
  });

  test('shows not-found message for missing shared result', async ({ page }) => {
    await page.goto('/results/00000000');
    await expect(page.locator('#errorView')).toBeVisible();
    await expect(page.locator('#resultView')).toHaveClass(/hidden/);
    await expect(page.locator('#errorView .error-message')).toContainText(/not found|expired/i);
  });

  test('shows server-error message when results API returns 500', async ({ page }) => {
    await page.route('**/api/v1/results/*', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'internal error' })
      });
    });

    await page.goto('/results/ABCDEF12');
    await expect(page.locator('#errorView')).toBeVisible();
    await expect(page.locator('#resultView')).toHaveClass(/hidden/);
    await expect(page.locator('#errorView .error-message')).toContainText(/server error/i);
  });

  test('shows timeout message when results API hangs', async ({ page }) => {
    await page.route('**/api/v1/results/*', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 25_000));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'ABCDEF12' })
      });
    });

    await page.goto('/results/ABCDEF12');
    await expect(page.locator('#errorView')).toBeVisible({ timeout: 30_000 });
    await expect(page.locator('#resultView')).toHaveClass(/hidden/);
    await expect(page.locator('#errorView .error-message')).toContainText(/timed out/i);
  });

  test('shows fallback error message for malformed results payload', async ({ page }) => {
    await page.route('**/api/v1/results/*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: '{not-json'
      });
    });

    await page.goto('/results/ABCDEF12');
    await expect(page.locator('#errorView')).toBeVisible();
    await expect(page.locator('#resultView')).toHaveClass(/hidden/);
    await expect(page.locator('#errorView .error-message')).toContainText(/unable to load result/i);
  });
});
