import { test, expect } from "./test";
import { VERSION, withAccounts } from "./fixtures";

test("splash shows the version and a keypress dismisses it", async ({ page, backend }) => {
  void backend; // installs the fake
  await page.goto("/");
  const splash = page.locator(".splash");
  await expect(splash).toBeVisible();
  await expect(splash.locator("pre")).toContainText(VERSION);
  await page.keyboard.press("Space");
  await expect(splash).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "PRAETOR" })).toBeVisible();
});

test("login form connects and reaches the game view", async ({ page, backend }) => {
  await backend.boot();
  await page.getByLabel("Username").fill("tester");
  await page.getByLabel("Password").fill("secret");
  await page.getByRole("button", { name: "Connect" }).click();
  await expect(backend.input).toBeVisible();
  expect(await backend.args("ConnectNew")).toEqual([["tester", "secret", true]]);
});

test.describe("with a stored account", () => {
  test.use({ init: withAccounts });

  test("account select connects the chosen account", async ({ page, backend }) => {
    await backend.boot();
    await expect(page.getByText("Choose an account")).toBeVisible();
    await page.getByRole("button", { name: "hero" }).click();
    await expect(backend.input).toBeVisible();
    expect(await backend.args("ConnectStored")).toEqual([["hero"]]);
  });
});
