import { defineConfig } from "@playwright/test";

// The harness has to be told where to listen before it starts, so the port is
// pinned here rather than chosen by the operating system. 8667 is one past the
// application's own 8666, which the harness proxies to.
const port = 8667;
const baseURL = `https://127.0.0.1:${port}`;

export default defineConfig({
  testDir: ".",
  // One account, one family, one database. The specs share a deployment, so
  // they run one at a time.
  workers: 1,
  fullyParallel: false,
  retries: 1,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  reporter: [["list"], ["html", { outputFolder: "../../build/playwright-report", open: "never" }]],
  outputDir: "../../build/playwright-results",

  use: {
    baseURL,
    // httptest signs the proxy's certificate itself, the way the reverse proxy
    // in front of production would not. A browser is right to object; this run
    // is not what the objection is for.
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },

  webServer: {
    // Built by `make test-ui`, not by `go run`, so the SIGTERM below reaches
    // the harness itself instead of the toolchain wrapper that spawned it.
    command: `build/e2e -serve -port ${port} -binary build/family_site`,
    cwd: "../..",
    url: `${baseURL}/readyz`,
    ignoreHTTPSErrors: true,
    timeout: 120_000,
    reuseExistingServer: false,
    stdout: "pipe",
    stderr: "pipe",
    // Without this the harness is killed outright, and the scratch deployment
    // it created stays behind to fail the next run's preflight.
    gracefulShutdown: { signal: "SIGTERM", timeout: 30_000 },
  },
});
