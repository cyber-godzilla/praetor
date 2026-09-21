import { test, expect } from "./test";
import { baseInit, largeInventoryLines, lines, makeConfig, styled } from "./fixtures";

const trimmedConfig = makeConfig();
trimmedConfig.UI.Scrollback = 1000;

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

test.describe("when a large block crosses the scrollback trim threshold", () => {
  test.use({ init: { ...baseInit, config: trimmedConfig } });

  test("a 333-line inventory containing gold tiaras stays at the tail", async ({ backend }) => {
    await backend.boot();
    await backend.connect();
    await backend.text(lines(1000));
    await expect
      .poll(() => backend.output.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight))
      .toBeLessThanOrEqual(2);

    const inventory = largeInventoryLines();
    expect(inventory).toHaveLength(333);
    await backend.text(inventory);

    await expect(backend.output.locator(".line", { hasText: "a gold tiara" })).toHaveCount(111);
    await expect(backend.output.getByText("End of inventory.")).toBeVisible();
    await expect
      .poll(() => backend.output.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight))
      .toBeLessThanOrEqual(2);
  });

  test("the same trim does not yank a deliberately detached viewport to the tail", async ({ backend }) => {
    await backend.boot();
    await backend.connect();
    await backend.text(lines(1000));
    await expect
      .poll(() => backend.output.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight))
      .toBeLessThanOrEqual(2);

    await backend.output.evaluate((el) => {
      el.scrollTop = 0;
      el.dispatchEvent(new Event("scroll"));
    });
    await backend.text(largeInventoryLines());
    await expect(backend.output.locator(".line")).toHaveCount(1076);
    expect(
      await backend.output.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight),
    ).toBeGreaterThan(1000);
  });
});
