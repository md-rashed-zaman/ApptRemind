import { defineConfig, devices } from "@playwright/test";

const noSandbox = process.env.PLAYWRIGHT_NO_SANDBOX === "1" || process.env.CI === "true";

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  fullyParallel: true,
  retries: 0,
  use: {
    baseURL: "http://127.0.0.1:3000",
    trace: "retain-on-failure",
    navigationTimeout: 60_000,
    launchOptions: noSandbox
      ? {
          args: ["--no-sandbox", "--disable-setuid-sandbox"]
        }
      : undefined
  },
  webServer: {
    command: "pnpm build && pnpm start",
    url: "http://127.0.0.1:3000",
    reuseExistingServer: true,
    timeout: 240_000,
    env: {
      PORT: "3000",
      HOSTNAME: "127.0.0.1"
    }
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] }
    }
  ]
});
