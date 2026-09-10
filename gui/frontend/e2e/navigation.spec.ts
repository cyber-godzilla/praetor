import { test, expect } from "./test";

test.beforeEach(async ({ backend }) => {
  await backend.boot();
  await backend.connect();
});

test("Ctrl+F searches the scrollback and Esc closes the bar", async ({ page, backend }) => {
  await backend.text(["alpha one", "beta two", "gamma three"]);
  await page.keyboard.press("Control+f");
  const search = page.getByPlaceholder("Find in scrollback…");
  await expect(search).toBeVisible();
  await expect(search).toBeFocused();
  await search.fill("beta");
  await expect(page.locator(".searchbar .count")).toHaveText("1/1");
  await expect(page.locator(".line.search-current")).toContainText("beta two");
  await page.keyboard.press("Escape");
  await expect(search).toHaveCount(0);
  await expect(backend.input).toBeFocused();
});

test("numpad 8 with NumLock off sends north", async ({ page, backend }) => {
  await backend.input.click();
  // Playwright cannot set NumLock; keyboard.press("Numpad8") reports key "8"
  // (NumLock ON). Dispatch exactly what WebKitGTK delivers with NumLock OFF.
  await page.evaluate(() => {
    (document.activeElement ?? window).dispatchEvent(
      new KeyboardEvent("keydown", { code: "Numpad8", key: "ArrowUp", bubbles: true, cancelable: true }),
    );
  });
  await expect.poll(() => backend.args("Send")).toEqual([["n"]]);
  await expect(backend.input).toHaveValue(""); // the arrow alias never reached the input
});
