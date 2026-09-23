import { test, expect } from "./test";
import { baseInit, makeConfig } from "./fixtures";

const topbarConfig = makeConfig();
topbarConfig.UI.DisplayMode = "topbar";

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
  await expect(dialog).toContainText("command && command");
  await expect(dialog.getByText("Send the syntax literally").locator("..").locator(".k"))
    .toHaveText(/\\\$\{\s+\\;;\s+\\&&/);
  await expect(dialog).toContainText("Commands");
});

test("Help can search game help and open the wiki", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Help", exact: true }).click();

  dialog = page.getByRole("dialog");
  await dialog.getByRole("textbox", { name: "Search game help" }).fill("parry");
  await dialog.getByRole("button", { name: "Search", exact: true }).click();
  await expect.poll(() => backend.args("Send")).toContainEqual(["?parry"]);

  await page.keyboard.press("Escape");
  dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Help", exact: true }).click();
  await dialog.getByRole("button", { name: "Open TEC Wiki" }).click();
  await expect.poll(() => backend.args("OpenURL")).toContainEqual([
    "http://eternal-city.wikidot.com",
  ]);
});

test("the sidebar Modes tab lists the loaded modes", async ({ page }) => {
  await page.locator(".sidebartabs .strip").getByRole("button", { name: "Modes" }).click();
  const modes = page.locator(".sidebartabs .modes");
  await expect(modes.getByRole("button", { name: "disable" })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt", exact: true })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt_wolves" })).toBeVisible();
});

test.describe("TUI-only configured topbar", () => {
  test.use({ init: { ...baseInit, config: topbarConfig } });

  test("falls back to the GUI sidebar without rewriting shared config", async ({ page, backend }) => {
    await page.setViewportSize({ width: 600, height: 700 });
    await expect(page.locator(".sidebar")).toBeVisible();
    await expect(page.locator(".game-topbar")).toHaveCount(0);
    expect(await backend.args("SetDisplayMode")).toEqual([]);
  });
});

test("Alt+S toggles and persists sidebar and off", async ({ page, backend }) => {
  const sidebar = page.locator(".sidebar");
  await expect(sidebar).toBeVisible();

  await page.keyboard.press("Alt+s");
  await expect(sidebar).toHaveCount(0);
  await expect.poll(() => backend.args("SetDisplayMode")).toEqual([["off"]]);

  await page.keyboard.press("Alt+s");
  await expect(sidebar).toBeVisible();
  await expect.poll(() => backend.args("SetDisplayMode")).toEqual([["off"], ["sidebar"]]);
});

test("GUI layout settings resize the sidebar and map", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Settings", exact: true }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByRole("spinbutton", { name: "Minimap Scale (Zoom)" }).fill("1.4");
  await dialog.getByRole("spinbutton", { name: "GUI sidebar width" }).fill("340");
  await dialog.getByRole("spinbutton", { name: "GUI map height" }).fill("220");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetMinimapScale")).toEqual([[1.4]]);
  await expect.poll(() => backend.args("SetGUILayout")).toEqual([[340, 220]]);
  await expect(page.locator(".sidebar")).toHaveCSS("width", "340px");
  await expect(page.locator(".sidebar .mapbox")).toHaveCSS("height", "220px");
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

test("the Escape menu Variables area saves command variables", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Variables", exact: true }).click();

  dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Reference them as ${name}");
  await dialog.getByRole("button", { name: "Add variable" }).click();
  await dialog.getByRole("textbox", { name: "Variable name" }).fill("destination");
  await dialog.getByRole("textbox", { name: "Value for destination" }).fill("north gate");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetInputVariables")).toEqual([
    [{ destination: "north gate" }],
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

test("notification permissions and sounds are controlled independently", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Notifications", exact: true }).click();

  dialog = page.getByRole("dialog");
  const scripts = dialog.getByRole("checkbox", { name: "Allow Script Notifications" });
  const sound = dialog.getByRole("checkbox", {
    name: "Play the OS default sound for notifications",
  });
  await expect(scripts).not.toBeChecked();
  await expect(sound).not.toBeChecked();
  await scripts.check();
  await sound.check();
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  const calls = await backend.args("SetNotifications");
  expect(calls).toHaveLength(1);
  expect(calls[0][0]).toMatchObject({ AllowScriptNotifications: true, Sound: true });
});

test("notification patterns save their custom message", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Notifications", exact: true }).click();

  dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Add pattern" }).click();
  await dialog.getByPlaceholder("match text").fill("whispers");
  await dialog.getByPlaceholder("title (optional)").fill("Private message");
  await dialog.getByPlaceholder("message (optional)").fill("Someone whispered to you");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetNotifications")).toEqual([
    [expect.objectContaining({
      Patterns: [{
        Pattern: "whispers",
        Title: "Private message",
        Message: "Someone whispered to you",
        Enabled: true,
      }],
    })],
  ]);
});

test("rank-bonus calculator compares targets and pages all twenty slots", async ({ page }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Rank-Bonus Calculator" }).click();
  dialog = page.getByRole("dialog");

  await dialog.getByRole("spinbutton", { name: "Current basics" }).fill("1150");
  await dialog.getByRole("spinbutton", { name: "Current subskill" }).fill("500");
  await dialog.getByRole("spinbutton", { name: "Target basics" }).fill("2000");
  await dialog.getByRole("spinbutton", { name: "Target subskill" }).fill("850");

  await expect(dialog.getByText("Current", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Target", { exact: true })).toBeVisible();
  await expect(dialog.getByText(/ΔBasics \+850/)).toBeVisible();
  await expect(dialog.getByText(/ΔSubskill \+350/)).toBeVisible();
  await expect(dialog.getByText(/Ranks 1151\+ require \/selftrain/)).toBeVisible();
  await expect(dialog.locator("table.matrix tbody tr")).toHaveCount(10);
  await expect(dialog.locator("table.matrix tbody tr").first()).toContainText("1");

  // Reproduce the compact window where the two rank tables previously spilled
  // into each other. They should stack and remain inside their own sections.
  await page.setViewportSize({ width: 755, height: 714 });
  const comparisonLayout = await dialog.locator(".comparisons").evaluate((comparison) => {
    const sections = Array.from(comparison.querySelectorAll<HTMLElement>(".rank-table"));
    const [current, target] = sections.map((section) => section.getBoundingClientRect());
    return {
      comparisonFits: comparison.scrollWidth <= comparison.clientWidth,
      sectionsFit: sections.every((section) => section.scrollWidth <= section.clientWidth),
      tablesStacked: target.top >= current.bottom,
    };
  });
  expect(comparisonLayout).toEqual({
    comparisonFits: true,
    sectionsFit: true,
    tablesStacked: true,
  });

  await dialog.getByRole("button", { name: "Show slots 11–20" }).click();
  await expect(dialog.locator("table.matrix tbody tr").first()).toContainText("11");
  await expect(dialog.locator("table.matrix tbody tr").last()).toContainText("20");
});
