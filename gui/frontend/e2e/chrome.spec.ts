import { dragAndDrop, test, expect } from "./test";
import { baseInit, COMPONENT_IDS, makeConfig } from "./fixtures";
import type { Page } from "@playwright/test";

async function openMenuItem(page: Page, section: string, item: string) {
  const dialog = page.getByRole("dialog");
  await dialog.locator("button.category").filter({ hasText: section }).click();
  await dialog.locator(".entry-list button").filter({ hasText: item }).click();
}

const topbarConfig = makeConfig();
topbarConfig.UI.DisplayMode = "topbar";

const constrainedWidthConfig = makeConfig();
const constrainedProfile = constrainedWidthConfig.DisplayProfiles.Profiles[0];
const constrainedSecondSidebar = constrainedProfile.Sidebars[1];
constrainedProfile.Main = constrainedProfile.Main.filter((component) => component.ComponentID !== COMPONENT_IDS.metrics);
constrainedProfile.Sidebars[0].Width = 400;
constrainedSecondSidebar.Enabled = true;
constrainedSecondSidebar.Width = 400;
constrainedSecondSidebar.Components = [{ ComponentID: COMPONENT_IDS.metrics, SidebarHeight: 240, Collapsed: false }];

const activeProfileCardInit = structuredClone(baseInit);
const activeProfileTemplate = activeProfileCardInit.config.CharacterProfiles.Profiles[0];
const activeProfileID = "866cc4c0-e476-4f8f-9f73-5a80554bb57c";
activeProfileCardInit.config.CharacterProfiles.Profiles.push(
  {
    ...structuredClone(activeProfileTemplate),
    ID: activeProfileID,
    Name: "Alpha",
    Account: "Example",
    CharacterNumber: 1,
  },
  {
    ...structuredClone(activeProfileTemplate),
    ID: "56c4bb73-d008-48dc-b6af-894e812aec72",
    Name: "Beta",
    Account: "Example",
    CharacterNumber: 3,
  },
);
activeProfileCardInit.activeCharacterProfileId = activeProfileID;
activeProfileCardInit.config.CharacterProfiles.Profiles[0].Variables = { target: "global target" };
activeProfileCardInit.config.CharacterProfiles.Profiles[1].Variables = { target: "local target" };

test.beforeEach(async ({ backend }) => {
  await backend.boot();
  await backend.connect();
});

test("Esc opens and closes the menu; Help opens from it", async ({ page }) => {
  const dialog = page.getByRole("dialog");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Quick actions");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);

  await page.keyboard.press("Escape");
  await openMenuItem(page, "Tools & help", "Help");
  await expect(dialog).toContainText("Key bindings");
  await expect(dialog).toContainText("command && command");
  await expect(dialog).toContainText('$(repeat "command" until "success" max 10)');
  await expect(dialog).toContainText('$(repeat "command" count 5)');
  await expect(dialog).toContainText("$(set-var name value)");
  await expect(dialog).toContainText("$(add-var name amount)");
  await expect(dialog).toContainText('cancel-on "text"');
  await expect(dialog).toContainText("/set-variable (/set-var)");
  await expect(dialog.getByText("Send the syntax literally").locator("..").locator(".k"))
    .toHaveText(/\\\$\{\s+\\\$\(\s+\\;;\s+\\&&/);
  await expect(dialog).toContainText("Commands");
});

test("the Escape menu presents quick actions and the stable settings hierarchy", async ({ page, backend }) => {
  await page.keyboard.press("Escape");

  const dialog = page.getByRole("dialog", { name: "Praetor" });
  await expect(dialog.getByText("Quick actions", { exact: true })).toBeVisible();
  await expect(dialog.locator("button.category")).toHaveText([
    /Character/,
    /Layout & display/,
    /Input & commands/,
    /Automation/,
    /Notifications & logs/,
    /Themes/,
    /Tools & help/,
    /Application/,
  ]);

  await expect(dialog.getByRole("button", { name: "Quit Praetor", exact: true })).toBeVisible();
  const reload = dialog.getByRole("button", { name: "Reload scripts", exact: true });
  await expect(reload).toBeVisible();
  await reload.click();
  await expect.poll(() => backend.calls("ReloadScripts")).toHaveLength(1);
  await expect(dialog).toBeVisible();

  await dialog.getByRole("searchbox", { name: "Search Praetor settings" }).fill("speech");
  const speechResult = dialog.locator(".search-results button").filter({ hasText: "Speech colors" });
  await expect(speechResult).toContainText("Layout & display");
  await speechResult.click();
  await expect(page.getByRole("dialog", { name: "Speech Colors" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Praetor settings" })).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "Praetor / Layout & display" })).toBeVisible();
  await page.keyboard.press("Escape");

  await page.getByRole("dialog", { name: "Praetor" }).locator("button.category").filter({ hasText: "Layout & display" }).click();
  const display = page.getByRole("dialog", { name: "Praetor / Layout & display" });
  await expect(display.locator(".entry-list button").filter({ hasText: "Arrange layout" })).toBeVisible();
  await expect(display.getByText("Current character", { exact: true }).first()).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "Praetor" })).toBeVisible();
});

test("Script Manager separates managed sources from local directories", async ({ page }) => {
  await page.keyboard.press("Escape");
  await openMenuItem(page, "Automation", "Script Manager");

  const dialog = page.getByRole("dialog", { name: "Script Manager" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Managed sources" })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Add praetor-scripts" })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Local directories" })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Save directories" })).toBeVisible();
});

test("Script Manager checks and applies an update with one button", async ({ page, backend }) => {
  const source = {
    id: "e093e10a-8990-41c2-bae5-dbd5e55cfe8e",
    name: "praetor-scripts",
    url: "https://github.com/cyber-godzilla/praetor-scripts.git",
    enabled: true,
    trust: "restricted",
    checkOnStartup: true,
    tracking: "tag",
    ref: "v0.2.6",
    installedCommit: "commit-026",
    installedVersion: "v0.2.6",
    status: {
      available: true,
      updated: false,
      version: "v0.3.0",
      commit: "commit-030",
      tracking: "tag",
      ref: "v0.3.0",
    },
    canEnable: true,
  };
  await backend.setReader("GetScriptManager", {
    localDirectories: [],
    managed: [source],
    suggestedURL: "https://github.com/cyber-godzilla/praetor-scripts.git",
    suggestedInstalled: true,
  });
  await backend.setReader("CheckManagedScript", {
    available: false,
    updated: true,
    version: "v0.3.0",
    commit: "commit-030",
    tracking: "tag",
    ref: "v0.3.0",
  });

  await page.keyboard.press("Escape");
  await openMenuItem(page, "Automation", "Script Manager");
  const dialog = page.getByRole("dialog", { name: "Script Manager" });
  const check = dialog.getByRole("button", { name: "Check for updates" });
  await expect(check).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Update", exact: true })).toHaveCount(0);
  await check.click();

  await expect.poll(() => backend.args("CheckManagedScript")).toContainEqual([source.id]);
  await expect(page.getByTestId("e2e-toasts")).toContainText("Script source updated");
  await expect(dialog.getByText("enabled", { exact: true })).toBeVisible();
});

test("character profiles can be added, renamed, numbered, and deleted", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  await openMenuItem(page, "Character", "Manage character profiles");

  let dialog = page.getByRole("dialog", { name: "Character Profiles" });
  await expect(dialog.locator(".default-name")).toContainText("Default");
  const autoSwitch = dialog.getByRole("checkbox", { name: "Automatically follow play and sleep commands" });
  await expect(autoSwitch).toBeChecked();
  await autoSwitch.uncheck();
  await dialog.getByRole("button", { name: "Add character profile" }).click();
  const rows = dialog.locator(".profile-row");
  await expect(rows).toHaveCount(2);
  await rows.nth(1).getByLabel("Profile name").fill("Marcus");
  await rows.nth(1).getByLabel("Account").fill("PlayerOne");
  await rows.nth(1).getByLabel("Character number").fill("1");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(async () => (await backend.args("SetCharacterProfiles")).length).toBe(1);
  await expect.poll(() => backend.args("SetCharacterProfileAutoSwitch")).toContainEqual([false]);
  const saved = (await backend.args("SetCharacterProfiles"))[0][0] as Array<Record<string, unknown>>;
  expect(saved).toHaveLength(2);
  expect(saved[1]).toMatchObject({ Name: "Marcus", Account: "PlayerOne", CharacterNumber: 1 });
  expect(saved[1].ID).toMatch(/^[0-9a-f]{8}-[0-9a-f-]{27}$/);

  await backend.events([{
    kind: "characterProfile",
    characterProfile: { id: saved[1].ID as string, name: "Marcus" },
  }]);
  await expect(page.locator(".tabbar").getByRole("button", { name: "Chat", exact: true })).toBeVisible();
  const characterChip = page.locator(".character-chip");
  await expect(characterChip).toContainText("Marcus");
  await characterChip.click();
  const characterMenu = page.getByRole("menu", { name: "Choose character profile" });
  await expect(characterMenu).toBeVisible();

  for (const button of await characterMenu.locator("button").all()) {
    const restingBackground = await button.evaluate((element) => getComputedStyle(element).backgroundColor);
    await button.hover();
    await expect.poll(() => button.evaluate((element) => getComputedStyle(element).backgroundColor))
      .not.toBe(restingBackground);
  }
  const activeProfile = characterMenu.getByRole("menuitemradio", { name: /Marcus/ });
  await activeProfile.hover();
  await expect.poll(() => activeProfile.evaluate((element) => {
    const detail = element.querySelector("small");
    return detail ? getComputedStyle(detail).color === getComputedStyle(element).color : false;
  })).toBe(true);

  const callsBeforeActiveSelection = (await backend.calls("SwitchCharacterProfile")).length;
  await activeProfile.click();
  await expect(characterMenu).not.toBeVisible();
  await expect.poll(async () => (await backend.calls("SwitchCharacterProfile")).length)
    .toBe(callsBeforeActiveSelection);

  await characterChip.click();
  await characterMenu.getByRole("menuitemradio", { name: /Default/ }).click();
  await expect.poll(() => backend.args("SwitchCharacterProfile")).toContainEqual([
    saved[0].ID,
    false,
  ]);
  await expect(characterChip).toContainText("Default");

  await page.keyboard.press("Escape");
  await openMenuItem(page, "Character", "Manage character profiles");
  dialog = page.getByRole("dialog", { name: "Character Profiles" });
  await expect(dialog.getByRole("checkbox", { name: "Automatically follow play and sleep commands" })).not.toBeChecked();
  await dialog.locator(".profile-row").nth(1).getByLabel("Profile name").fill("Marcellus");
  page.once("dialog", (confirmation) => confirmation.accept());
  await dialog.locator(".profile-row").nth(1).getByRole("button", { name: "Delete profile" }).click();
  await expect(dialog.locator(".profile-row")).toHaveCount(1);
});

test.describe("active character profile card", () => {
  test.use({ init: activeProfileCardInit });

  test("keeps the same field geometry as inactive cards", async ({ page }) => {
    await page.keyboard.press("Escape");
    await openMenuItem(page, "Character", "Manage character profiles");

    const dialog = page.getByRole("dialog", { name: "Character Profiles" });
    const characterRows = dialog.locator('.profile-row:has(input[aria-label="Profile name"])');
    const activeWidths = await characterRows.nth(0).locator("input").evaluateAll((inputs) =>
      inputs.map((input) => input.getBoundingClientRect().width),
    );
    const inactiveWidths = await characterRows.nth(1).locator("input").evaluateAll((inputs) =>
      inputs.map((input) => input.getBoundingClientRect().width),
    );

    expect(activeWidths).toEqual(inactiveWidths);
  });

  test("does not add a use-global control to character variables", async ({ page }) => {
    const variables = page.locator('aside[aria-label="Sidebar 1"] .variables');
    await expect(variables.getByLabel("Value for target")).toHaveValue("local target");
    await expect(variables.getByRole("button", { name: /use global/i })).toHaveCount(0);
  });
});

test("Help can search game help and open the wiki", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await openMenuItem(page, "Tools & help", "Help");

  dialog = page.getByRole("dialog");
  await dialog.getByRole("textbox", { name: "Search game help" }).fill("parry");
  await dialog.getByRole("button", { name: "Search", exact: true }).click();
  await expect.poll(() => backend.args("Send")).toContainEqual(["?parry"]);

  await page.keyboard.press("Escape");
  dialog = page.getByRole("dialog");
  await openMenuItem(page, "Tools & help", "Help");
  await dialog.getByRole("button", { name: "Open TEC Wiki" }).click();
  await expect.poll(() => backend.args("OpenURL")).toContainEqual([
    "https://tec-wiki.com",
  ]);
});

test("the sidebar Modes tab lists the loaded modes", async ({ page }) => {
  const modes = page.locator('aside[aria-label="Sidebar 1"] .modes');
  await expect(modes.getByRole("button", { name: "disable" })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt", exact: true })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt_wolves" })).toBeVisible();
});

test("sidebar action buttons add and remove columns with available width", async ({ page }) => {
  const sidebar = page.locator('aside[aria-label="Sidebar 1"]');
  const actions = sidebar.locator(".section").filter({ hasText: "Action Sets" });
  const buttons = actions.locator(".buttons");
  const columnCount = () => buttons.evaluate((element) =>
    getComputedStyle(element).gridTemplateColumns.split(" ").filter(Boolean).length,
  );

  await sidebar.evaluate((element) => { element.style.width = "180px"; });
  await expect.poll(columnCount).toBe(1);

  await sidebar.evaluate((element) => { element.style.width = "260px"; });
  await expect.poll(columnCount).toBe(2);

  await sidebar.evaluate((element) => { element.style.width = "600px"; });
  await expect.poll(columnCount).toBeGreaterThan(2);
});

test("sidebar sections retain the 0.5 frame-title collapse control", async ({ page, backend }) => {
  const section = page.locator('aside[aria-label="Sidebar 1"] .section').filter({ hasText: "Map" });
  const toggle = section.getByRole("button", { name: "Map", exact: true });
  await expect(toggle.locator("svg.chevron")).toBeVisible();
  await expect(toggle).toHaveCSS("position", "absolute");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(section.locator(".content")).toHaveCount(0);
  await expect.poll(() => backend.args("SetViewLayout")).not.toHaveLength(0);
});

test("reloading scripts immediately refreshes mode listings and hint breadcrumbs", async ({ page, backend }) => {
  const refreshed = [
    { name: "disable", usage: "", desc: "Turn automation off", chains: false },
    { name: "new_mode", usage: "<target>", desc: "Freshly loaded mode", chains: true },
  ];
  await backend.setReader("ModeNames", refreshed.map((m) => m.name));
  await backend.setReader("ModeSpecs", refreshed);

  await page.keyboard.press("Escape");
  const dialog = page.getByRole("dialog");
  await dialog.locator("button.category").filter({ hasText: "Automation" }).click();
  await dialog.locator(".entry-list button").filter({ hasText: "Reload scripts" }).click();
  await expect.poll(() => backend.calls("ReloadScripts")).toHaveLength(1);
  // The always-mounted Modes view refreshes once on startup and once here.
  await expect.poll(() => backend.calls("ModeNames")).toHaveLength(2);
  await expect.poll(() => backend.calls("ModeSpecs")).toHaveLength(1);
  await dialog.getByLabel("Close").click();

  const modes = page.locator('aside[aria-label="Sidebar 1"] .modes');
  await expect(modes.getByRole("button", { name: "new_mode" })).toBeVisible();
  await expect(modes.getByRole("button", { name: "hunt", exact: true })).toHaveCount(0);

  await backend.input.fill("/mode new_mode ");
  const hint = page.getByTestId("e2e-hint");
  await expect(hint).toContainText("Freshly loaded mode");
  await expect(hint).toContainText("<target>");
  await expect(hint).toContainText(
    "after_<mode|do|ps>:<mode|command|praetorscript>",
  );

  await backend.input.fill("/mode hunt ");
  await expect(hint).not.toContainText("Hunt the given target");
});

test.describe("TUI-only configured topbar", () => {
  test.use({ init: { ...baseInit, config: topbarConfig } });

  test("uses the responsive Views drawer without rewriting shared config", async ({ page, backend }) => {
    await page.setViewportSize({ width: 600, height: 700 });
    await expect(page.getByRole("button", { name: "Views" })).toBeVisible();
    await expect(page.locator(".game-topbar")).toHaveCount(0);
    expect(await backend.args("SetDisplayMode")).toEqual([]);
  });
});

test.describe("width-constrained sidebars", () => {
  test.use({ init: { ...baseInit, config: constrainedWidthConfig }, viewport: { width: 1280, height: 720 } });

  test("collapses a sidebar to an actionable title without rendering its views", async ({ page, backend }) => {
    const sidebar1 = page.locator('aside[aria-label="Sidebar 1"]');
    await expect(sidebar1).toBeVisible();
    await expect(page.locator('aside[aria-label="Sidebar 2"]')).toBeVisible();

    await sidebar1.getByRole("button", { name: "Collapse Sidebar 1" }).click();
    await expect(sidebar1).toHaveCSS("width", "30px");
    await expect(sidebar1.locator(".section")).toHaveCount(0);
    await expect.poll(async () => {
      const calls = await backend.args("SetViewLayout");
      const saved = calls.at(-1)?.[0] as { Sidebars?: Array<{ Collapsed?: boolean }> } | undefined;
      return saved?.Sidebars?.[0]?.Collapsed;
    }).toBe(true);
    await sidebar1.getByRole("button", { name: "Expand Sidebar 1" }).click();
    await expect(sidebar1).toHaveCSS("width", "400px");
    await expect(sidebar1.locator(".section")).not.toHaveCount(0);
    await expect.poll(async () => {
      const calls = await backend.args("SetViewLayout");
      const saved = calls.at(-1)?.[0] as { Sidebars?: Array<{ Collapsed?: boolean }> } | undefined;
      return !!saved?.Sidebars?.[0]?.Collapsed;
    }).toBe(false);

    await page.setViewportSize({ width: 1100, height: 720 });

    const collapsed = page.locator('aside[aria-label="Sidebar 2"]');
    await expect(collapsed).toBeVisible();
    await expect(collapsed).toHaveCSS("width", "30px");
    await expect(collapsed.getByRole("button", { name: "Expand Sidebar 2" })).toBeVisible();
    await expect(collapsed.locator(".section")).toHaveCount(0);
    await expect(collapsed).not.toContainText("Metrics");
    await expect(page.getByTestId("e2e-toasts")).toContainText("Sidebar collapsed");
    await expect(page.getByTestId("e2e-toasts")).toContainText("Sidebar 2 was collapsed because the window is too narrow.");

    await collapsed.getByRole("button", { name: "Expand Sidebar 2" }).click();
    await expect(collapsed).toHaveCSS("width", "400px");
    await expect(collapsed).toContainText("Metrics");
    await expect(sidebar1).toHaveCSS("width", "30px");
  });
});

test("Alt+S toggles and persists sidebar and off", async ({ page, backend }) => {
  const sidebar = page.locator('aside[aria-label="Sidebar 1"]');
  await expect(sidebar).toBeVisible();

  await page.keyboard.press("Alt+s");
  await expect(sidebar).toHaveCount(0);
  await expect.poll(() => backend.args("SetGUISidebarsVisible")).toEqual([[false]]);

  await page.keyboard.press("Alt+s");
  await expect(sidebar).toBeVisible();
  await expect.poll(() => backend.args("SetGUISidebarsVisible")).toEqual([[false], [true]]);
});

test("relocated input and display settings persist from their categories", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await openMenuItem(page, "Input & commands", "Input behavior");
  dialog = page.getByRole("dialog");
  await dialog.getByRole("spinbutton", { name: "Command chain delay" }).fill("1250");
  await dialog.getByRole("spinbutton", { name: "Unbusy response delay" }).fill("250");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await page.keyboard.press("Escape");
  await openMenuItem(page, "Layout & display", "Text & display settings");
  dialog = page.getByRole("dialog");
  await dialog.getByRole("spinbutton", { name: "Minimap Scale (Zoom)" }).fill("1.4");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetSemicolonDelay")).toEqual([[1250]]);
  await expect.poll(() => backend.args("SetUnbusyDelay")).toEqual([[250]]);
  await expect.poll(() => backend.args("SetMinimapScale")).toEqual([[1.4]]);
});

test("Layout drag-and-drop immediately saves UUID sidebar placement and dimensions", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog", { name: "Praetor" });
  await openMenuItem(page, "Layout & display", "Arrange layout");
  dialog = page.getByRole("dialog", { name: "Layout", exact: true });

  const sidebar1 = dialog.getByRole("group", { name: "Sidebar 1" });
  await sidebar1.getByRole("spinbutton", { name: "Sidebar 1 width" }).fill("340");
  const sidebar2 = dialog.getByRole("group", { name: "Sidebar 2" });
  await sidebar2.getByRole("checkbox", { name: "Sidebar 2 enabled" }).check();
  await sidebar2.getByRole("combobox", { name: "Sidebar 2 side" }).selectOption("left");
  await sidebar1.getByRole("spinbutton", { name: "Map height" }).fill("220");
  const metrics = dialog.locator(".main-destination .component").filter({ hasText: "Metrics" });
  const map = sidebar1.locator(".component").filter({ hasText: "Map" });
  const mapBox = await map.boundingBox();
  const transfer = await page.evaluateHandle(() => new DataTransfer());
  await metrics.dispatchEvent("dragstart", { dataTransfer: transfer });
  await map.dispatchEvent("dragover", {
    dataTransfer: transfer,
    clientX: mapBox!.x + mapBox!.width / 2,
    clientY: mapBox!.y + 1,
  });
  await expect(map).toHaveClass(/drop-before/);
  await metrics.dispatchEvent("dragend", { dataTransfer: transfer });
  await expect(map).not.toHaveClass(/drop-before/);
  await dragAndDrop(page, metrics, sidebar2.locator(".destination"));
  await dialog.getByRole("button", { name: "+ Add sidebar" }).click();
  await expect(dialog.getByRole("group", { name: "Sidebar 4" })).toBeVisible();
  await dialog.locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("dialog", { name: "Praetor / Layout & display" }).getByLabel("Close").click();

  await expect.poll(async () => (await backend.args("SetViewLayout")).length).toBeGreaterThanOrEqual(5);
  await expect(page.locator('aside[aria-label="Sidebar 1"]')).toHaveCSS("width", "340px");
  await expect(page.locator('aside[aria-label="Sidebar 2"]')).toBeVisible();
  await expect(page.locator('aside[aria-label="Sidebar 2"]')).toContainText("Metrics");
});

test("Layout Bench lists only unplaced components and receives deleted sidebars", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog", { name: "Praetor" });
  await openMenuItem(page, "Layout & display", "Arrange layout");
  dialog = page.getByRole("dialog", { name: "Layout", exact: true });

  const bench = dialog.getByRole("list", { name: "View Bench" });
  const main = dialog.locator(".main-destination");
  const [benchBox, mainBox] = await Promise.all([
    dialog.locator(".bench-section").boundingBox(),
    dialog.locator(".main-section").boundingBox(),
  ]);
  expect(benchBox!.x).toBeGreaterThan(mainBox!.x + mainBox!.width);
  await expect(dialog.locator("code")).toHaveCount(0);
  await expect(dialog.getByText("Debug", { exact: true })).toHaveCount(0);
  await expect(main.getByRole("listitem").filter({ hasText: "All" })).toContainText("Type: Game output");
  await expect(main.getByRole("listitem").filter({ hasText: "Chat" })).toContainText("Type: Filtered text");
  await expect(bench.getByText("All views are placed.")).toBeVisible();
  await expect(bench.getByRole("listitem")).toHaveCount(0);
  await expect(dialog.getByRole("combobox", { name: /Component ID to add/ })).toHaveCount(0);
  await expect(main.locator("code")).toHaveCount(0);

  await dialog.getByRole("button", { name: "Delete Sidebar 1" }).click();
  await expect(dialog.getByRole("group", { name: "Sidebar 1" })).toHaveCount(0);
  await expect(bench.getByRole("listitem")).toHaveCount(6);
  await expect(bench.locator("code")).toHaveCount(0);
  await expect(bench.getByRole("listitem").filter({ hasText: "Map" })).toContainText("Type: Map");
  await dialog.getByRole("textbox", { name: "Filter benched views" }).fill("Compass");
  await expect(bench.getByRole("listitem")).toHaveCount(1);
  await expect(bench.getByRole("listitem")).toHaveText(/Compass/);
  await dialog.getByRole("textbox", { name: "Filter benched views" }).fill("");

  await expect(dialog.locator(".component-browser")).toHaveCSS("overflow-y", "auto");
  const [benchAfterDelete, dialogBox] = await Promise.all([
    dialog.locator(".bench-section").boundingBox(),
    dialog.boundingBox(),
  ]);
  expect(benchAfterDelete!.y + benchAfterDelete!.height).toBeLessThanOrEqual(dialogBox!.y + dialogBox!.height + 1);

  await expect.poll(async () => (await backend.args("SetViewLayout")).length).toBeGreaterThan(0);
  let calls = await backend.args("SetViewLayout");
  let saved = calls.at(-1)?.[0] as { Main: Array<{ ComponentID: string }>; Bench: Array<{ ComponentID: string }>; Sidebars: Array<{ Name: string }> };
  expect(saved.Sidebars.some((sidebar) => sidebar.Name === "Sidebar 1")).toBe(false);
  expect(saved.Main.some((component) => component.ComponentID === COMPONENT_IDS.map)).toBe(false);
  expect(saved.Bench.slice(-6).map((component) => component.ComponentID)).toEqual([
    COMPONENT_IDS.map,
    COMPONENT_IDS.exits,
    COMPONENT_IDS.vitals,
    COMPONENT_IDS.actions,
    COMPONENT_IDS.modes,
    COMPONENT_IDS.variables,
  ]);

  await bench.getByRole("listitem").filter({ hasText: "Map" }).dragTo(main);
  await expect(bench.getByRole("listitem")).toHaveCount(5);
  await expect.poll(async () => (await backend.args("SetViewLayout")).length).toBeGreaterThan(calls.length);
  calls = await backend.args("SetViewLayout");
  saved = calls.at(-1)?.[0] as typeof saved;
  expect(saved.Main.some((component) => component.ComponentID === COMPONENT_IDS.map)).toBe(true);
  expect(saved.Bench.some((component) => component.ComponentID === COMPONENT_IDS.map)).toBe(false);
});

test("View Library creates filtered text views on the Layout Bench", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog", { name: "Praetor" });
  await openMenuItem(page, "Layout & display", "View Library");
  dialog = page.getByRole("dialog", { name: "View Library" });

  await expect(dialog.getByText("Filtered text views", { exact: true })).toBeVisible();
  await dialog.getByPlaceholder("New filtered text view…").fill("Thoughts");
  await dialog.getByRole("button", { name: "Add filtered text view" }).click();

  await expect.poll(async () => (await backend.args("SetProfileViewLayout")).length).toBeGreaterThan(0);
  const calls = await backend.args("SetProfileViewLayout");
  const [, savedLayout, savedViews] = calls.at(-1)! as [
    string,
    { Bench: Array<{ ComponentID: string }> },
    Array<{ ID: string; Name: string }>,
  ];
  const thoughts = savedViews.find((view) => view.Name === "Thoughts");
  expect(thoughts).toBeDefined();
  expect(savedLayout.Bench.some((placement) => placement.ComponentID === thoughts!.ID)).toBe(true);

  await dialog.locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
  dialog = page.getByRole("dialog", { name: "Praetor / Layout & display" });
  await dialog.locator(".entry-list button").filter({ hasText: "Arrange layout" }).click();
  dialog = page.getByRole("dialog", { name: "Layout", exact: true });
  await expect(dialog.getByRole("list", { name: "View Bench" }).getByRole("listitem").filter({ hasText: "Thoughts" })).toBeVisible();
  await expect(dialog.getByText("Filtered text views", { exact: true })).toHaveCount(0);
});

test("the sidebar Variables tab saves command variables", async ({ page, backend }) => {
  const sidebar = page.locator('aside[aria-label="Sidebar 1"]');
  await sidebar.getByRole("button", { name: "Add variable" }).click();
  await page.getByRole("textbox", { name: "Variable name" }).fill("target");
  await page.getByRole("textbox", { name: "Value for target" }).fill("scarred bandit");
  await page.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetInputVariables")).toEqual([
    [{ target: "scarred bandit" }],
  ]);
});

test("saved variable events live-update an open Variables tab", async ({ page, backend }) => {
  const sidebar = page.locator('aside[aria-label="Sidebar 1"]');
  await expect(sidebar.getByText("No variables configured.")).toBeVisible();

  await backend.events([{
    kind: "variable",
    variable: { name: "target", value: "scarred bandit" },
  }]);
  await expect(page.getByRole("textbox", { name: "Variable name" })).toHaveValue("target");
  await expect(page.getByRole("textbox", { name: "Value for target" })).toHaveValue("scarred bandit");

  await backend.events([{
    kind: "variable",
    variable: { name: "target", value: "hooded figure" },
  }]);
  await expect(page.getByRole("textbox", { name: "Value for target" })).toHaveValue("hooded figure");
});

test("the Escape menu Variables area saves command variables", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await openMenuItem(page, "Input & commands", "Variables");

  dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Reference them as ${name}");
  await expect(dialog).toContainText("${name:fallback}");
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

  await page.getByRole("button", { name: "Wait for cue" }).click();
  await expect.poll(() => backend.args("SendInput")).toEqual([
    ["attack ${target};;look"],
    ["$(wait-for \"ready\")"],
  ]);
});

test("targeted speech is submitted with its unmatched leading quote", async ({ backend }) => {
  await backend.input.fill('"apula Good morning');
  await backend.input.press("Enter");

  await expect.poll(() => backend.args("SendInput")).toEqual([
    ['"apula Good morning'],
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

test("a speech line can seed a character in the speech-color picker", async ({ page, backend }) => {
  await backend.text([
    {
      text: 'Apula says, "Keep moving."',
      segments: [{ text: 'Apula says, "Keep moving."' }],
      speechSpeaker: "Apula",
      timestamp: 0,
    },
    "The wind blows.",
  ]);

  await backend.output.getByText("The wind blows.", { exact: true }).click({ button: "right" });
  await expect(page.getByRole("menuitem", { name: "Set a speech color" })).toHaveCount(0);

  await backend.output.getByText('Apula says, "Keep moving."', { exact: true }).click({ button: "right" });
  await page.getByRole("menuitem", { name: "Set a speech color" }).click();

  const settings = page.getByRole("dialog", { name: "Speech Colors" });
  await expect(settings.getByRole("textbox", { name: "Character pattern" })).toHaveValue("Apula");
  await expect(settings.getByRole("checkbox", { name: "Include think for Apula" })).not.toBeChecked();
  await expect(settings.getByRole("checkbox", { name: "Include OOC for Apula" })).not.toBeChecked();
  await settings.getByRole("checkbox", { name: "Include think for Apula" }).check();
  await settings.getByRole("checkbox", { name: "Include OOC for Apula" }).check();
  await settings.locator("button.style-choice").click();

  const picker = page.getByRole("dialog", { name: "Choose a Colorword style" });
  const search = picker.getByRole("combobox", { name: "Search color names or families" });
  await search.fill("red");
  await expect(picker.getByRole("option", { name: /Scarlet/ })).toBeVisible();
  await expect(picker.getByRole("option", { name: /Crimson/ })).toBeVisible();
  await search.fill("crim");
  await expect(picker.getByRole("option", { name: /Crimson/ })).toBeVisible();
  await expect(picker.getByRole("option", { name: /Scarlet/ })).toHaveCount(0);
  await picker.getByRole("option", { name: /Crimson/ }).click();
  await settings.getByRole("textbox", { name: "Character pattern" }).fill("${speaker:Apula}");

  await settings.getByRole("button", { name: "Save", exact: true }).click();
  await expect.poll(() => backend.args("SetSpeechColors")).toEqual([[
    [{
      Character: "${speaker:Apula}",
      Style: "crimson",
      Active: true,
      IncludeThink: true,
      IncludeOOC: true,
    }],
  ]]);
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
  await openMenuItem(page, "Notifications & logs", "Notifications");

  dialog = page.getByRole("dialog");
  const scripts = dialog.getByRole("checkbox", { name: "Enable script notifications" });
  const sound = dialog.locator(".script-card").getByRole("checkbox", { name: "Sound" });
  await expect(scripts).not.toBeChecked();
  await expect(sound).not.toBeChecked();
  await scripts.check();
  await sound.check();
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  const calls = await backend.args("SetNotifications");
  expect(calls).toHaveLength(1);
  expect(calls[0][0]).toMatchObject({
	Desktop: { AllowScriptNotifications: true, Sound: true, ScriptSound: true },
	Discord: { Enabled: false },
  });
});

test("notification patterns save their custom message", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await openMenuItem(page, "Notifications & logs", "Notifications");

  dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Add string match" }).click();
  await dialog.getByPlaceholder("text, * ? or ${name}").fill("whispers");
  await dialog.getByLabel("Title").fill("Private message");
  await dialog.getByLabel("Message").fill("Someone whispered to you");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();

  await expect.poll(() => backend.args("SetNotifications")).toEqual([
    [expect.objectContaining({
	  Desktop: expect.objectContaining({
		Patterns: [{
		  Pattern: "whispers",
		  Title: "Private message",
		  Message: "Someone whispered to you",
		  Enabled: true,
		  When: "always",
		  Sound: false,
		  Delivery: "desktop",
		}],
	  }),
    })],
  ]);
});

test("Discord notifications are masked, categorized, and testable", async ({ page, backend }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await openMenuItem(page, "Notifications & logs", "Notifications");

  dialog = page.getByRole("dialog");
  const webhook = dialog.getByLabel("Webhook URL");
  await expect(webhook).toHaveAttribute("type", "password");
  await webhook.fill("https://discord.com/api/webhooks/123/secret-token");
  await dialog.getByRole("checkbox", { name: "Enable Discord delivery" }).check();
  await dialog.getByLabel("Health notification options").getByRole("combobox", { name: "Send to" }).selectOption("both");
  await dialog.getByRole("checkbox", { name: "Also send to Discord" }).check();
  await dialog.getByRole("button", { name: "Save and send test" }).click();

  await expect.poll(() => backend.args("SetNotifications")).toEqual([
	[expect.objectContaining({
	  Discord: {
		Enabled: true,
		WebhookURL: "https://discord.com/api/webhooks/123/secret-token",
		Health: true,
		Fatigue: false,
		Patterns: false,
		Scripts: true,
	  },
	})],
  ]);
  await expect.poll(() => backend.calls("TestDiscordNotification")).toHaveLength(1);
});

test("rank-bonus calculator compares targets and pages all twenty slots", async ({ page }) => {
  await page.keyboard.press("Escape");
  let dialog = page.getByRole("dialog");
  await openMenuItem(page, "Tools & help", "Rank-bonus calculator");
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
