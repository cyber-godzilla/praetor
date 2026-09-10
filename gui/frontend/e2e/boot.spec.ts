import { test, expect } from "@playwright/test";

// Task 1 uses the bare Playwright `test`; Task 2 switches this file to the
// fake-backend fixture from ./test once it exists.
test("splash shows the version and a keypress dismisses it", async ({ page }) => {
  await page.goto("/");
  const splash = page.locator(".splash");
  await expect(splash).toBeVisible();
  // Outside Wails the bridge degrades and GetInitState reports version "dev".
  await expect(splash.locator("pre")).toContainText("dev");
  await page.keyboard.press("Space");
  await expect(splash).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "PRAETOR" })).toBeVisible();
});
