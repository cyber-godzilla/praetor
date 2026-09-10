import { defineConfig, devices } from "@playwright/test";

// Smoke suite for the GUI frontend. Runs against the BUILT assets (vite
// preview over dist/) — the same files Wails embeds — never the dev server.
// The Wails bridge is faked per test by e2e/fake-backend.ts; nothing here
// touches the real game server.
export default defineConfig({
  testDir: "e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://localhost:4173",
    trace: "retain-on-failure",
    viewport: { width: 1280, height: 800 },
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    // build:fast skips svelte-check (make -C gui check already runs it).
    command: "npm run build:fast && npx vite preview --port 4173 --strictPort",
    url: "http://localhost:4173",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
