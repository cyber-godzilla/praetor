import { test, expect } from "./test";

test.beforeEach(async ({ backend }) => {
  await backend.boot();
  await backend.connect();
});

test("Esc opens and closes the menu; Help opens from it", async ({ page }) => {
  const dialog = page.getByRole("dialog");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Menu");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);

  await page.keyboard.press("Escape");
  await dialog.getByRole("button", { name: "Help", exact: true }).click();
  await expect(dialog).toContainText("Key bindings");
  await expect(dialog).toContainText("Commands");
});

test("the sidebar Modes tab lists the loaded modes", async ({ page }) => {
  await page.locator(".sidebartabs .strip").getByRole("button", { name: "Modes" }).click();
  const modes = page.locator(".sidebartabs .modes");
  await expect(modes.getByRole("button", { name: "disable" })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt", exact: true })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt_wolves" })).toBeVisible();
});

test("the sidebar Variables tab saves command variables", async ({ page, backend }) => {
  await page.locator(".sidebartabs .strip").getByRole("button", { name: "Variables" }).click();
  await page.getByRole("button", { name: "Add variable" }).click();
  await page.getByRole("textbox", { name: "Variable name" }).fill("target");
  await page.getByRole("textbox", { name: "Value for target" }).fill("scarred bandit");
  await page.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetInputVariables")).toEqual([
    [{ target: "scarred bandit" }],
  ]);
});

test("action-set commands use live variable substitution and command chaining", async ({ page, backend }) => {
  await page.getByRole("button", { name: "Attack target" }).click();
  await expect.poll(() => backend.args("SendInput")).toEqual([
    ["attack ${target};;look"],
  ]);
});

test("a custom tab routes matching lines and Tab cycles tabs", async ({ page, backend }) => {
  const tabbar = page.locator(".tabbar");
  await expect(tabbar.getByRole("button", { name: /^Chat/ })).toBeVisible();

  await backend.text(['Bob says, "hello"', "The wind blows."]);
  await tabbar.getByRole("button", { name: /^Chat/ }).click();
  await expect(tabbar.locator("button.active")).toHaveText(/^Chat/);
  await expect(backend.output).toContainText("Bob says");
  await expect(backend.output).not.toContainText("The wind blows.");

  await page.keyboard.press("Tab");
  await expect(tabbar.locator("button.active")).toHaveText("Metrics");
  await page.keyboard.press("Shift+Tab");
  await expect(tabbar.locator("button.active")).toHaveText(/^Chat/);
});

test("a notify event shows a toast", async ({ page, backend }) => {
  await backend.events([{ kind: "notify", notify: { title: "Ping", message: "someone waved" } }]);
  const toasts = page.getByTestId("e2e-toasts");
  await expect(toasts).toContainText("Ping");
  await expect(toasts).toContainText("someone waved");
});

test("notification sounds are controlled by one global switch", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Notifications", exact: true }).click();

  dialog = page.getByRole("dialog");
  const sound = dialog.getByRole("checkbox", {
    name: "Play the OS default sound for notifications",
  });
  await expect(sound).not.toBeChecked();
  await sound.check();
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  const calls = await backend.args("SetNotifications");
  expect(calls).toHaveLength(1);
  expect(calls[0][0]).toMatchObject({ Sound: true });
});
