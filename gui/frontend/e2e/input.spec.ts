import { test, expect } from "./test";
import type { Locator } from "@playwright/test";

async function bounds(locator: Locator) {
  const result = await locator.boundingBox();
  if (!result) throw new Error("expected visible element to have a bounding box");
  return result;
}

test.beforeEach(async ({ backend }) => {
  await backend.boot();
  await backend.connect();
});

test("spellcheck setting reaches the command input", async ({ backend }) => {
  await expect(backend.input).toHaveAttribute("spellcheck", "false");
});

test("Enter sends the typed command and clears the input", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("look");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("SendInput")).toEqual([["look"]]);
  await expect(backend.input).toHaveValue("");
});

test("typed mode arguments reach the shared command parser unchanged", async ({ page, backend }) => {
  await backend.input.fill("/mode unlock_all sack");
  await page.keyboard.press("Enter");
  await backend.input.fill('/mode unlock_all from:"2 sack"');
  await page.keyboard.press("Enter");

  await expect.poll(() => backend.args("SendInput")).toEqual([
    ["/mode unlock_all sack"],
    ['/mode unlock_all from:"2 sack"'],
  ]);
  expect(await backend.args("SetMode")).toEqual([]);
});

test("the Automation Bar gives PraetorScript the full space left of its controls", async ({ page, backend }) => {
  const controls = page.getByTestId("input-controls");
  const placeholder = page.getByTestId("chain-status-placeholder");
  const play = page.getByRole("button", { name: /play/i });
  const mode = page.getByTitle("Switch mode");

  await expect(placeholder).toHaveText("PraetorScript idle");
  await expect(play).toBeVisible();
  await expect(mode).toBeVisible();
  await expect(page.getByRole("button", { name: "Stop command chain" })).toHaveCount(0);

  const inputBox = await bounds(backend.input);
  const controlsBox = await bounds(controls);
  const placeholderBox = await bounds(placeholder);
  const playBox = await bounds(play);
  const idleModeBox = await bounds(mode);
  expect(controlsBox.y).toBeGreaterThanOrEqual(inputBox.y + inputBox.height - 1);
  expect(placeholderBox.x).toBeCloseTo(controlsBox.x + 12, 0);
  expect(placeholderBox.x + placeholderBox.width).toBeCloseTo(playBox.x - 8, 0);

  await backend.input.fill("stand&&look");
  await page.keyboard.press("Enter");

  const stop = page.getByRole("button", { name: "Stop command chain" });
  await expect(stop).toBeVisible();
  await expect(placeholder).toHaveCount(0);
  await expect(page.getByTestId("chain-status")).toContainText("2/2");
  await expect(page.getByTestId("chain-status")).toContainText("waiting for unbusy");
  await expect(play).toHaveCount(0);

  const activeStatusBox = await bounds(page.getByTestId("chain-status"));
  const stopBox = await bounds(stop);
  const activeModeBox = await bounds(mode);
  expect(activeStatusBox.x).toBeCloseTo(controlsBox.x + 12, 0);
  expect(activeStatusBox.x + activeStatusBox.width).toBeCloseTo(stopBox.x - 8, 0);
  expect(activeModeBox.x).toBeCloseTo(idleModeBox.x, 0);

  await stop.click();
  await expect.poll(() => backend.args("AbortInputChains")).toEqual([[]]);
  await expect(stop).toHaveCount(0);
  await expect(play).toBeVisible();
  await expect(placeholder).toHaveText("PraetorScript idle");
});

test("the Automation Bar stays below both one-line and five-line input", async ({ page, backend }) => {
  const controls = page.getByTestId("input-controls");
  const mode = page.getByTitle("Switch mode");
  const oneLineInput = await bounds(backend.input);
  const oneLineControls = await bounds(controls);
  const oneLineMode = await bounds(mode);
  expect(oneLineControls.y).toBeGreaterThanOrEqual(oneLineInput.y + oneLineInput.height - 1);

  await backend.input.fill("one\ntwo\nthree\nfour\nfive");
  await expect(backend.input).toHaveValue("one\ntwo\nthree\nfour\nfive");

  const fiveLineInput = await bounds(backend.input);
  const fiveLineControls = await bounds(controls);
  const fiveLineMode = await bounds(mode);
  expect(fiveLineInput.height).toBeGreaterThan(oneLineInput.height + 40);
  expect(fiveLineControls.y).toBeGreaterThanOrEqual(fiveLineInput.y + fiveLineInput.height - 1);
  expect(fiveLineMode.x).toBeCloseTo(oneLineMode.x, 0);
  await expect(page.getByTestId("chain-status-placeholder")).toHaveText("PraetorScript idle");
});

test("a standalone PraetorScript control exposes the chain stop control", async ({ page, backend }) => {
  await backend.input.fill("$(wait-for \"ready\" timeout 30)");
  await page.keyboard.press("Enter");

  const stop = page.getByRole("button", { name: "Stop command chain" });
  await expect(stop).toBeVisible();
  await expect(page.getByTestId("chain-status")).toContainText("waiting for “ready”");
  await expect(page.getByTestId("chain-status")).toContainText("30s");
  await expect.poll(() => backend.args("SendInput")).toEqual([["$(wait-for \"ready\" timeout 30)"]]);

  await stop.click();
  await expect.poll(() => backend.args("AbortInputChains")).toEqual([[]]);
});

test("paced chains show their fixed starting delay", async ({ page, backend }) => {
  await backend.input.fill("look;;inventory");
  await page.keyboard.press("Enter");

  const status = page.getByTestId("chain-status");
  await expect(status).toContainText("2/2");
  await expect(status).toContainText("pacing");
  await expect(status).toContainText("0.9s");
});

test("/guide opens the welcome wiki links without navigating automatically", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("/guide");
  await page.keyboard.press("Enter");

  await expect(page.getByText("Welcome to Praetor", { exact: true })).toBeVisible();
  expect(await backend.args("OpenURL")).toEqual([]);
});

test("Shift+Enter inserts a newline; Enter sends the block as one message", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("say hello");
  await page.keyboard.press("Shift+Enter");
  await page.keyboard.type("there");
  await expect(backend.input).toHaveValue("say hello\nthere");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("SendInput")).toEqual([["say hello\nthere"]]);
});

test("Ctrl+R filters history and Enter sends the match", async ({ page, backend }) => {
  await backend.input.click();
  await page.keyboard.type("look");
  await page.keyboard.press("Enter");
  await page.keyboard.type("get sword");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("SendInput")).toEqual([["look"], ["get sword"]]);

  await page.keyboard.press("Control+r");
  await expect(page.locator(".rsearch")).toBeVisible();
  await page.keyboard.type("lo");
  await expect(page.locator(".rsearch")).toContainText("lo");
  await expect(backend.input).toHaveValue("look");
  await page.keyboard.press("Enter");
  await expect.poll(() => backend.args("SendInput")).toEqual([["look"], ["get sword"], ["look"]]);
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
  expect(await backend.args("SendInput")).toEqual([]);
});
