const { defineConfig } = require('@playwright/test');

// GHA: cap parallelism (typical 2 vCPU); reduces contention vs default. Local: Playwright default (CPU-based).
// Override in any env: PLAYWRIGHT_WORKERS=1 bunx playwright test
const workers =
  process.env.PLAYWRIGHT_WORKERS != null && process.env.PLAYWRIGHT_WORKERS !== ''
    ? Number.parseInt(process.env.PLAYWRIGHT_WORKERS, 10)
    : process.env.GITHUB_ACTIONS
      ? 2
      : undefined;

if (
  process.env.PLAYWRIGHT_WORKERS != null &&
  process.env.PLAYWRIGHT_WORKERS !== '' &&
  (!Number.isFinite(workers) || workers < 1)
) {
  throw new Error(
    `invalid PLAYWRIGHT_WORKERS: ${JSON.stringify(process.env.PLAYWRIGHT_WORKERS)}`
  );
}

module.exports = defineConfig({
  testDir: 'test/e2e/ui',
  timeout: 90_000,
  expect: {
    timeout: 15_000
  },
  workers,
  use: {
    baseURL: 'http://127.0.0.1:8080',
    trace: 'on-first-retry',
    viewport: { width: 1280, height: 720 }
  },
  webServer: {
    command: 'BIND_ADDRESS=127.0.0.1 PORT=8080 ALLOWED_ORIGINS=* go run ./cmd/openbyte server',
    url: 'http://127.0.0.1:8080/health',
    timeout: 120_000,
    reuseExistingServer: true
  }
});
