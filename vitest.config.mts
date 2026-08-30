import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: { "@app": new URL("./frontend", import.meta.url).pathname },
  },
  test: {
    include: ["frontend/**/*.test.ts", "frontend/**/*.test.tsx"],
    environment: "node",
    setupFiles: ["frontend/test-setup.ts"],
    // Date formatting reads the local zone, so an unpinned zone makes the same
    // assertion pass in CI and fail on a developer's machine.
    env: { TZ: "UTC" },
  },
});
