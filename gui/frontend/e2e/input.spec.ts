import { test, expect } from "./test";

test.beforeEach(async ({ backend }) => {
  await backend.boot();
  await backend.connect();
});

test("Enter sends the typed command and clears the input", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("look");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("Send")).toEqual([["look"]]);
  await expect(backend.input).toHaveValue("");
});

test("Shift+Enter inserts a newline; Enter sends the block as one message", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("say hello");
  await page.keyboard.press("Shift+Enter");
  await page.keyboard.type("there");
  await expect(backend.input).toHaveValue("say hello\nthere");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("Send")).toEqual([["say hello\nthere"]]);
});

test("Ctrl+R filters history and Enter sends the match", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("look");
  await page.keyboard.press("Enter");
  await page.keyboard.type("get sword");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("Send")).toEqual([["look"], ["get sword"]]);

  await page.keyboard.press("Control+r");
  await expect(page.locator(".rsearch")).toBeVisible();
  await page.keyboard.type("lo");
  await expect(page.locator(".rsearch")).toContainText("lo");
  await expect(backend.input).toHaveValue("look");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("Send")).toEqual([["look"], ["get sword"], ["look"]]);
  await expect(page.locator(".rsearch")).toHaveCount(0);
});

test("/mode hint lists the loaded modes and Tab completes a unique prefix", async ({ page, backend }) => {
  const hint = page.getByTestId("e2e-hint");
  await backend.input.click();
  await page.keyboard.type("/mode ");
  await expect(hint).toBeVisible();
  await expect(hint).toContainText("hunt");
  await expect(hint).toContainText("hunt_wolves");
  await expect(hint).toContainText("disable");
  await page.keyboard.type("hunt_w");
  await expect(hint).not.toContainText("disable");
  await page.keyboard.press("Tab");
  await expect(backend.input).toHaveValue(/^\/mode hunt_wolves/);
  // completion never sends. This is a synchronous read, not expect.poll: a
  // poll's first sample would pass trivially even if completion secretly
  // queued a send, because the preceding `toHaveValue` round trip already
  // flushed the microtask chain a stray send would ride — polling again
  // afterward proves nothing a synchronous check doesn't already prove.
  expect(await backend.args("Send")).toEqual([]);
});
