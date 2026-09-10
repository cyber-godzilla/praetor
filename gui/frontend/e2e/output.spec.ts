import { test, expect } from "./test";
import { lines, styled } from "./fixtures";

test("game text renders with its segment styles", async ({ backend }) => {
  await backend.boot();
  await backend.connect();
  await backend.text(["You are standing in the forum.", styled]);
  await expect(backend.output).toContainText("You are standing in the forum.");
  await expect(backend.output.locator("span", { hasText: "bold" })).toHaveCSS("font-weight", "700");
  await expect(backend.output.locator("span", { hasText: "italic" })).toHaveCSS("font-style", "italic");
  await expect(backend.output.locator("span", { hasText: "red" })).toHaveCSS("color", "rgb(255, 0, 0)");
});

test("a 2000-line burst leaves the pane at the tail", async ({ backend }) => {
  await backend.boot();
  await backend.connect();
  await backend.text(lines(2000));
  await expect(backend.output.getByText("line 2000")).toBeVisible();
  // Distance from the bottom, in px; the follow logic must keep it at ~0.
  await expect
    .poll(() => backend.output.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight))
    .toBeLessThanOrEqual(2);
  // Sanity: the pane really is scrollable, so the assertion above means something.
  expect(await backend.output.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
});
