import { defineConfig } from "@playwright/test";

const baseURL = process.env.KOMARI_BASE_URL || "http://127.0.0.1:25775";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  timeout: 120000,
  expect: { timeout: 20000 },
  retries: 0,
  reporter: [["list"]],
  globalSetup: "./global-setup.js",
  globalTeardown: "./global-teardown.js",
  use: {
    baseURL,
    locale: "en-US",
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
});
