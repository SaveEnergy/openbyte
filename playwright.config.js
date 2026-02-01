const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: 'test/e2e/ui',
  timeout: 90_000,
  expect: {
    timeout: 15_000
  },
  use: {
    baseURL: 'http://127.0.0.1:8080',
    trace: 'on-first-retry',
    viewport: { width: 1280, height: 720 }
  },
  webServer: {
    command: 'BIND_ADDRESS=127.0.0.1 PORT=8080 WEB_ROOT=./web ALLOWED_ORIGINS=* go run ./cmd/server',
    url: 'http://127.0.0.1:8080/health',
    timeout: 120_000,
    reuseExistingServer: true
  }
});
