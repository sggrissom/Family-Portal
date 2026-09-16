import { defineConfig } from "@playwright/test";

// Pinned, because the harness is told where to listen before it starts. 8667 is
// one past the application's own 8666, which the harness proxies to.
const port = 8667;
const baseURL = `https://127.0.0.1:${port}`;

export default defineConfig({
  testDir: ".",
  // The specs share one deployment and one database, so they run one at a time.
  workers: 1,
  fullyParallel: false,
  retries: 1,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  reporter: [["list"], ["html", { outputFolder: "../../build/playwright-report", open: "never" }]],
  outputDir: "../../build/playwright-results",

  use: {
    baseURL,
    // httptest signs the proxy's certificate itself.
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },

  webServer: {
    // Built by `make test-ui`, not run with `go run`, so the SIGTERM below
    // reaches the harness rather than the toolchain wrapper around it.
    command: `build/e2e -serve -port ${port} -binary build/family_site`,
    cwd: "../..",
    url: `${baseURL}/readyz`,
    ignoreHTTPSErrors: true,
    timeout: 120_000,
    reuseExistingServer: false,
    stdout: "pipe",
    stderr: "pipe",
    // Without this the harness is killed outright and the scratch deployment it
    // created stays behind, failing the next run's preflight.
    gracefulShutdown: { signal: "SIGTERM", timeout: 30_000 },
  },
});
