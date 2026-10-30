import { readFileSync } from "node:fs";
import { mkdir } from "node:fs/promises";
import { basename, resolve } from "node:path";
import { dragAndDrop, test, expect } from "./test";
import { baseInit, makeConfig } from "./fixtures";
import type { Page } from "@playwright/test";
import type { ImagePayload, ModeSpec, RBResult, TextPayload, TrainingCostRow, WireEvent } from "../src/lib/types";

const SCREENSHOT_DIR = resolve("artifacts/ui-screenshots");
const GRAPHICS_DIR = resolve(SCREENSHOT_DIR, "graphics");

async function openMenuItem(page: Page, section: string, item: string) {
  const dialog = page.getByRole("dialog");
  await dialog.locator("button.category").filter({ hasText: section }).click();
  await dialog.locator(".entry-list button").filter({ hasText: item }).click();
}

function generatedPNG(path: string): ImagePayload {
  const png = readFileSync(path);
  const signature = png.subarray(0, 8).toString("hex");
  if (signature !== "89504e470d0a1a0a" || png.toString("ascii", 12, 16) !== "IHDR") {
    throw new Error(`${basename(path)} is not a PNG with an IHDR header`);
  }
  return {
    dataURI: `data:image/png;base64,${png.toString("base64")}`,
    width: png.readUInt32BE(16),
    height: png.readUInt32BE(20),
  };
}

const minimap = generatedPNG(resolve(GRAPHICS_DIR, "minimap.png"));
const compass = generatedPNG(resolve(GRAPHICS_DIR, "compass.png"));
const rbCalcFixture = JSON.parse(
  readFileSync(resolve(SCREENSHOT_DIR, "rbcalc.json"), "utf8"),
) as {
  current: RBResult;
  target: RBResult;
  costs: TrainingCostRow[];
};

const docsModeSpecs: ModeSpec[] = [
  { name: "disable", usage: "", desc: "Stop all automation", chains: false },
  {
    name: "loot",
    usage: "<item|alias> [start:<corpse#>] [from:<noun>] [stow:<container>] [stow_start:<n>] [drop:<item|list>]",
    desc: "Take a pipe-delimited item list from every corpse, rotating stowage containers",
    chains: true,
  },
  { name: "wagon", usage: "<alias|items> [vendor] [container]", desc: "Sell the contents of a wagon to a vendor", chains: true },
  { name: "idle", usage: "", desc: "Wait for fatigue to recover, then chain onward", chains: true },
  { name: "macro", usage: "[nokill]", desc: "Rotate six attack macros against the current target", chains: false },
];

const docsConfig = makeConfig();
docsConfig.CharacterProfiles.Profiles[0].Variables = {
  count: "5",
  target: "bandit",
  weapon: "gladius",
};
docsConfig.Notifications.Desktop = {
  AllowScriptNotifications: true,
  Sound: false,
  ScriptSound: false,
  HealthBelow: { Enabled: true, Threshold: 25, When: "unfocused", Sound: true, Delivery: "both" },
  FatigueBelow: { Enabled: false, Threshold: 10, When: "always", Sound: false, Delivery: "desktop" },
  Patterns: [
    { Pattern: "retalq", Title: "Rare drop", Message: "Retalq appeared in the output.", Enabled: true, When: "unfocused", Sound: true, Delivery: "desktop" },
  ],
};

test.use({
  init: {
    ...baseInit,
    modeNames: docsModeSpecs.map((mode) => mode.name),
    modeSpecs: docsModeSpecs,
    config: docsConfig,
  },
});

function line(text: string, color?: string): TextPayload {
  return { text, segments: [{ text, ...(color ? { color } : {}) }], timestamp: 0 };
}

function segmented(segments: TextPayload["segments"]): TextPayload {
  return {
    text: segments.map((segment) => segment.text).join(""),
    segments,
    timestamp: 0,
  };
}

const sampleOutput: TextPayload[] = [
  line("Welcome to the Welcome Room! This cozy little place might be ideal, whether you're just passing through or stopping by for a chat. A fountain trickles nearby, surrounded by several comfortable-looking chairs. High on the wall is a small window that looks out on a garden. In front of the southeast corner is a glowing red portal. Before a glass display case is an automated auction sign. A large promotional advertisement sign hangs from the ceiling by a thread."),
  segmented([
    { text: "You are facing east. You see " },
    { text: "Quiet Room A", color: "#646464" },
    { text: " to the " },
    { text: "north", color: "#646464" },
    { text: "; a " },
    { text: "walkway", color: "#646464" },
    { text: " to the " },
    { text: "east", color: "#646464" },
    { text: " and to the " },
    { text: "west", color: "#646464" },
    { text: "; " },
    { text: "Quiet Room B", color: "#646464" },
    { text: " to the " },
    { text: "south", color: "#646464" },
    { text: "; and an " },
    { text: "obsidian staircase", color: "#646464" },
    { text: " leading " },
    { text: "downwards", color: "#646464" },
    { text: "." },
  ]),
  line("A softly glowing golden plaque and a table are scattered about ahead of you."),
  line(""),
  line("No new forum messages found."),
];

const sampleState: WireEvent[] = [
  {
    kind: "bars",
    bars: {
      hasHealth: true, health: 60,
      hasFatigue: true, fatigue: 39,
      hasEncumbrance: true, encumbrance: 50,
      hasSatiation: true, satiation: 87,
      hasLighting: true, lighting: 0, lightingRaw: 150,
    },
  },
  { kind: "status", status: { mode: "disable" } },
  { kind: "minimap", image: minimap },
  { kind: "compass", image: compass },
];

async function addLayoutCallouts(page: Page): Promise<void> {
  const sidebarSections = page.locator('aside[aria-label="Sidebar 1"] .section');
  const regions = [
    { number: 1, target: page.locator(".statusbar"), badgeX: 0.66, badgeY: 0.5 },
    { number: 2, target: page.locator(".tabbar"), badgeX: 0.59, badgeY: 0.5 },
    { number: 3, target: page.locator(".output"), badgeX: 0.97, badgeY: 0.96 },
    { number: 4, target: sidebarSections.nth(0), badgeX: 0.94, badgeY: 0.08 },
    { number: 5, target: sidebarSections.nth(1), badgeX: 0.94, badgeY: 0.08 },
    { number: 6, target: sidebarSections.nth(2), badgeX: 0.94, badgeY: 0.08 },
    { number: 7, target: page.locator('aside[aria-label="Sidebar 1"]'), badgeX: 0.94, badgeY: 0.06 },
    { number: 8, target: page.locator(".inputbar"), badgeX: 0.78, badgeY: 0.5 },
    { number: 9, target: page.locator(".automation-bar"), badgeX: 0.79, badgeY: 0.5 },
  ];

  const callouts = [];
  for (const { target, ...region } of regions) {
    const box = await target.boundingBox();
    if (!box) throw new Error(`layout callout target ${region.number} is not visible`);
    callouts.push({ ...region, ...box });
  }

  await page.evaluate((items) => {
    const namespace = "http://www.w3.org/2000/svg";
    const overlay = document.createElementNS(namespace, "svg");
    overlay.id = "docs-layout-callouts";
    overlay.setAttribute("viewBox", `0 0 ${window.innerWidth} ${window.innerHeight}`);
    Object.assign(overlay.style, {
      position: "fixed",
      inset: "0",
      width: "100vw",
      height: "100vh",
      pointerEvents: "none",
      zIndex: "2147483647",
    });

    for (const item of items) {
      const rect = document.createElementNS(namespace, "rect");
      rect.setAttribute("x", String(item.x + 1));
      rect.setAttribute("y", String(item.y + 1));
      rect.setAttribute("width", String(Math.max(0, item.width - 2)));
      rect.setAttribute("height", String(Math.max(0, item.height - 2)));
      rect.setAttribute("fill", "none");
      rect.setAttribute("stroke", "#ffd400");
      rect.setAttribute("stroke-width", "2");
      overlay.append(rect);

      const badgeX = Math.min(item.x + item.width - 13, Math.max(item.x + 13, item.x + item.width * item.badgeX));
      const badgeY = Math.min(item.y + item.height - 13, Math.max(item.y + 13, item.y + item.height * item.badgeY));
      const badge = document.createElementNS(namespace, "circle");
      badge.setAttribute("cx", String(badgeX));
      badge.setAttribute("cy", String(badgeY));
      badge.setAttribute("r", "11");
      badge.setAttribute("fill", "#ffd400");
      badge.setAttribute("stroke", "#111116");
      badge.setAttribute("stroke-width", "1.5");
      overlay.append(badge);

      const label = document.createElementNS(namespace, "text");
      label.setAttribute("x", String(badgeX));
      label.setAttribute("y", String(badgeY + 0.5));
      label.setAttribute("fill", "#111116");
      label.setAttribute("font-family", "sans-serif");
      label.setAttribute("font-size", "13");
      label.setAttribute("font-weight", "700");
      label.setAttribute("text-anchor", "middle");
      label.setAttribute("dominant-baseline", "middle");
      label.textContent = String(item.number);
      overlay.append(label);
    }

    document.body.append(overlay);
  }, callouts);
}

test("render documentation screenshots from deterministic sample data", async ({ page, backend }) => {
  await page.setViewportSize({ width: 1200, height: 800 });
  await backend.boot();

  // This is only a synthetic state transition. It does not call ConnectNew or
  // ConnectStored and therefore cannot open a socket to the game server.
  await backend.enterGame();
  await backend.events(sampleState);
  await backend.text(sampleOutput);

  await expect(page.locator('aside[aria-label="Sidebar 1"] img')).toHaveCount(2);
  await expect(page.getByText("A softly glowing golden plaque and a table are scattered about ahead of you.", { exact: true })).toBeVisible();
  await expect(page.getByTestId("chain-status-placeholder")).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  await mkdir(SCREENSHOT_DIR, { recursive: true });

  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-main.png"),
    animations: "disabled",
    caret: "hide",
  });

  await addLayoutCallouts(page);
  await expect(page.locator("#docs-layout-callouts")).toBeVisible();
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-layout-callouts.png"),
    animations: "disabled",
    caret: "hide",
  });
  await page.locator("#docs-layout-callouts").evaluate((element) => element.remove());

  await backend.setInputChainStatus({
    active: true,
    chains: 1,
    step: 2,
    total: 4,
    state: "wait-for",
    detail: "You stop pulling",
    durationMs: 30_000,
    remainingMs: 27_000,
  });
  await expect(page.getByTestId("chain-status")).toContainText("You stop pulling");
  await expect(page.getByRole("button", { name: "Stop command chain" })).toBeVisible();

  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-automation.png"),
    animations: "disabled",
    caret: "hide",
  });

  expect(await backend.calls("ConnectNew")).toHaveLength(0);
  expect(await backend.calls("ConnectStored")).toHaveLength(0);

  // The real sidebar keeps all ten movement targets over the generated image;
  // in particular, the center is split into independent up/down controls.
  await page.getByRole("button", { name: "Go up" }).click();
  await page.getByRole("button", { name: "Go down" }).click();
  await expect.poll(() => backend.args("Send")).toContainEqual(["up"]);
  await expect.poll(() => backend.args("Send")).toContainEqual(["down"]);

  // Capture the current menu and its most documentation-heavy subviews. These
  // use the real components and the same deterministic config as the main
  // frame, so the wiki cannot silently drift to hand-built mockups.
  await page.setViewportSize({ width: 1200, height: 900 });
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toContainText("Quick actions");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-menu.png"),
    animations: "disabled",
    caret: "hide",
  });

  await openMenuItem(page, "Input & commands", "Input behavior");
  await expect(page.getByRole("dialog")).toContainText(";; chain delay (ms)");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-settings.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to Input & commands").click();
  await page.getByRole("dialog").getByRole("button", { name: "Notifications & logs", exact: true }).click();
  await page.getByRole("dialog").locator(".entry-list button").filter({ hasText: "Notifications" }).click();
  await expect(page.getByRole("dialog")).toContainText("Only while unfocused");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-notifications.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to Notifications & logs").click();
  await page.getByRole("dialog").getByRole("button", { name: "Input & commands", exact: true }).click();
  await page.getByRole("dialog").locator(".entry-list button").filter({ hasText: "Variables" }).click();
  await expect(page.getByRole("dialog", { name: "Variables" }).getByLabel("Value for target")).toHaveValue("bandit");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-variables.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to Input & commands").click();
  await page.getByRole("dialog").getByRole("button", { name: "Tools & help", exact: true }).click();
  await page.getByRole("dialog").locator(".entry-list button").filter({ hasText: "Rank-bonus calculator" }).click();
  await backend.setReader("CalcRankBonus:0:1150:500", rbCalcFixture.current);
  await backend.setReader("CalcRankBonus:0:1150:1150", rbCalcFixture.target);
  await backend.setReader(
    "CalcTrainingCosts:1150:500:1150:1150:false:false:false",
    rbCalcFixture.costs,
  );
  await page.getByLabel("Current basics").fill("1150");
  await page.getByLabel("Current subskill").fill("500");
  await page.getByLabel("Target basics").fill("1150");
  await page.getByLabel("Target subskill").fill("1150");
  await expect(page.getByText("Training cost", { exact: true })).toBeVisible();
  const rankTables = page.getByRole("dialog").locator(".rank-table");
  await expect(rankTables.nth(0)).toContainText("Basics RB 168 · Subskill RB 154");
  await expect(rankTables.nth(0).locator("tbody tr").first()).toContainText("Defensive168280238196170.8");
  await expect(rankTables.nth(1)).toContainText("Basics RB 168 · Subskill RB 168");
  await expect(rankTables.nth(1).locator("tbody tr").first()).toContainText("Defensive168294252210184.8");
  await expect(page.getByRole("dialog").locator("table.matrix tbody tr").first()).toContainText("103250455058507150");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-rbcalc.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to Tools & help").click();
  await page.getByLabel("Close").click();
  await backend.input.fill("/list");
  await backend.input.press("Enter");
  await expect(page.getByRole("dialog")).toContainText("rotating stowage containers");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-mode-select.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByLabel("Close").click();
  await backend.setInputChainStatus({ active: false, chains: 0, step: 0, total: 0, state: "" });
  await backend.input.fill("/mode loot ");
  await expect(page.getByTestId("e2e-hint")).toContainText("<item|alias>");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-mode-hint.png"),
    animations: "disabled",
    caret: "hide",
  });
});

test("render one, two, and three sidebar layout examples with the real controls", async ({ page, backend }) => {
  test.slow();
  await page.setViewportSize({ width: 1600, height: 900 });
  await backend.boot();
  await backend.enterGame();
  await backend.events(sampleState);
  await backend.text(sampleOutput);
  await page.evaluate(() => document.fonts.ready);
  await mkdir(SCREENSHOT_DIR, { recursive: true });

  async function openLayout() {
    const menu = page.getByRole("dialog", { name: "Praetor" });
    if (!(await menu.isVisible())) await page.keyboard.press("Escape");
    await openMenuItem(page, "Layout & display", "Arrange layout");
    return page.getByRole("dialog", { name: "Layout", exact: true });
  }

  async function saveLayout() {
    await page.getByRole("dialog", { name: "Layout", exact: true }).locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "Layout", exact: true })).toHaveCount(0);
    await page.getByRole("dialog", { name: "Praetor / Layout & display" }).getByLabel("Close").click();
    await expect(page.getByTestId("e2e-toasts").locator(".toast")).toHaveCount(0, { timeout: 7_000 });
  }

  async function setSidebar(
    dialog: ReturnType<Page["getByRole"]>,
    id: number,
    enabled: boolean,
    side: "left" | "right",
    width: number,
  ) {
    const group = dialog.getByRole("group", { name: `Sidebar ${id}` });
    const checkbox = group.getByRole("checkbox", { name: `Sidebar ${id} enabled` });
    if (enabled) await checkbox.check();
    else await checkbox.uncheck();
    await group.getByRole("combobox", { name: `Sidebar ${id} side` }).selectOption(side);
    await group.getByRole("spinbutton", { name: `Sidebar ${id} width` }).fill(String(width));
  }

  async function place(
    dialog: ReturnType<Page["getByRole"]>,
    view: string,
    location: "main" | 1 | 2 | 3,
    height: number,
  ) {
    const unplace = dialog.getByRole("button", { name: `Unplace ${view}` });
    if (await unplace.count()) await unplace.click();
    const destination = location === "main"
      ? dialog.locator(".main-destination")
      : dialog.getByRole("group", { name: `Sidebar ${location}` }).locator(".destination");
    const source = dialog.getByRole("list", { name: "View Bench" })
      .getByRole("listitem").filter({ hasText: view });
    const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
    await source.dispatchEvent("dragstart", { dataTransfer });
    await destination.dispatchEvent("dragover", { dataTransfer });
    await destination.dispatchEvent("drop", { dataTransfer });
    await dataTransfer.dispose();
    if (location !== "main") await destination.getByRole("spinbutton", { name: `${view} height` }).fill(String(height));
  }

  // One sidebar: navigation and status on the right, with automation views as tabs.
  let dialog = await openLayout();
  await setSidebar(dialog, 1, true, "right", 300);
  await setSidebar(dialog, 2, false, "left", 230);
  await setSidebar(dialog, 3, false, "right", 230);
  await place(dialog, "Map", 1, 250);
  await place(dialog, "Compass", 1, 170);
  await place(dialog, "Vitals", 1, 150);
  await place(dialog, "Action Sets", "main", 190);
  await place(dialog, "Modes", "main", 210);
  await place(dialog, "Variables", "main", 210);
  await saveLayout();
  await expect(page.locator('aside[aria-label="Sidebar 1"]')).toBeVisible();
  await expect(page.locator('aside[aria-label^="Sidebar"]')).toHaveCount(1);
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-layout-1-sidebar.png"),
    animations: "disabled",
    caret: "hide",
  });

  // Two sidebars: data and variables on the left, navigation and combat on the right.
  dialog = await openLayout();
  await setSidebar(dialog, 2, true, "left", 230);
  await place(dialog, "Metrics", 2, 210);
  await place(dialog, "Variables", 2, 230);
  await place(dialog, "Map", 1, 180);
  await place(dialog, "Compass", 1, 140);
  await place(dialog, "Vitals", 1, 120);
  await place(dialog, "Action Sets", 1, 150);
  await saveLayout();
  await expect(page.locator('aside[aria-label="Sidebar 2"]')).toBeVisible();
  await expect(page.locator('aside[aria-label^="Sidebar"]')).toHaveCount(2);
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-layout-2-sidebars.png"),
    animations: "disabled",
    caret: "hide",
  });

  // Three sidebars: a second right-hand rail holds the automation controls.
  dialog = await openLayout();
  await setSidebar(dialog, 3, true, "right", 230);
  await place(dialog, "Action Sets", 3, 210);
  await place(dialog, "Modes", 3, 260);
  await saveLayout();
  await expect(page.locator('aside[aria-label="Sidebar 3"]')).toBeVisible();
  await expect(page.locator('aside[aria-label^="Sidebar"]')).toHaveCount(3);
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-layout-3-sidebars.png"),
    animations: "disabled",
    caret: "hide",
  });
});

test("render a width-collapsed sidebar and its explicit Expand control", async ({ page, backend }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await backend.boot();
  await backend.enterGame();
  await backend.events(sampleState);
  await backend.text(sampleOutput);
  await page.evaluate(() => document.fonts.ready);
  await mkdir(SCREENSHOT_DIR, { recursive: true });

  await page.keyboard.press("Escape");
  await openMenuItem(page, "Layout & display", "Arrange layout");
  const dialog = page.getByRole("dialog", { name: "Layout", exact: true });
  const sidebar1 = dialog.getByRole("group", { name: "Sidebar 1" });
  const sidebar2 = dialog.getByRole("group", { name: "Sidebar 2" });
  await sidebar1.getByRole("spinbutton", { name: "Sidebar 1 width" }).fill("400");
  await sidebar2.getByRole("checkbox", { name: "Sidebar 2 enabled" }).check();
  await sidebar2.getByRole("spinbutton", { name: "Sidebar 2 width" }).fill("400");
  await dragAndDrop(
    page,
    dialog.locator(".main-destination .component").filter({ hasText: "Metrics" }),
    sidebar2.locator(".destination"),
  );
  await dialog.locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
  await page.getByLabel("Close").click();

  await page.setViewportSize({ width: 1100, height: 800 });
  const collapsed = page.locator('aside[aria-label="Sidebar 2"]');
  await expect(collapsed).toHaveCSS("width", "30px");
  await expect(collapsed.locator(".section")).toHaveCount(0);
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-sidebar-collapsed.png"),
    animations: "disabled",
    caret: "hide",
  });

  await collapsed.getByRole("button", { name: "Expand Sidebar 2" }).click();
  await expect(collapsed).toContainText("Metrics");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-sidebar-expanded.png"),
    animations: "disabled",
    caret: "hide",
  });
});

test("render a filtered Think and OOC custom view in a sidebar", async ({ page, backend }) => {
  await page.setViewportSize({ width: 1600, height: 900 });
  await backend.boot();
  await backend.enterGame();
  await backend.events(sampleState);
  await backend.text(sampleOutput);

  // Configure placement with Layout and define the reusable filtered view in
  // the View Library, following the same split exposed to users.
  await page.keyboard.press("Escape");
  await openMenuItem(page, "Layout & display", "Arrange layout");
  let dialog = page.getByRole("dialog", { name: "Layout", exact: true });

  const sidebar1 = dialog.getByRole("group", { name: "Sidebar 1" });
  await sidebar1.getByRole("spinbutton", { name: "Sidebar 1 width" }).fill("300");
  const sidebar2 = dialog.getByRole("group", { name: "Sidebar 2" });
  await sidebar2.getByRole("checkbox", { name: "Sidebar 2 enabled" }).check();
  await sidebar2.getByRole("combobox", { name: "Sidebar 2 side" }).selectOption("left");
  await sidebar2.getByRole("spinbutton", { name: "Sidebar 2 width" }).fill("360");

  await sidebar1.getByRole("spinbutton", { name: "Map height" }).fill("250");
  await sidebar1.getByRole("spinbutton", { name: "Compass height" }).fill("170");
  await sidebar1.getByRole("spinbutton", { name: "Vitals height" }).fill("150");
  const bench = dialog.getByRole("list", { name: "View Bench" });
  const main = dialog.locator(".main-destination");
  for (const view of ["Action Sets", "Modes", "Variables"] as const) {
    await dialog.getByRole("button", { name: `Unplace ${view}` }).click();
    await dragAndDrop(page, bench.getByRole("listitem").filter({ hasText: view }), main);
  }

  await dialog.locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("dialog", { name: "Praetor / Layout & display" }).locator(".entry-list button").filter({ hasText: "View Library" }).click();
  dialog = page.getByRole("dialog", { name: "View Library" });

  await dialog.getByPlaceholder("New filtered text view…").fill("Think / OOC");
  await dialog.getByRole("button", { name: "Add filtered text view" }).click();

  const editor = dialog.locator(".view-editor");
  await editor.getByRole("button", { name: "+ Add rule" }).click();
  await editor.locator(".rule").nth(0).getByPlaceholder("pattern").fill("thinks aloud");
  await editor.getByRole("button", { name: "+ Add rule" }).click();
  await editor.locator(".rule").nth(1).getByPlaceholder("pattern").fill("You think aloud");
  await editor.getByRole("button", { name: "+ Add rule" }).click();
  await editor.locator(".rule").nth(2).getByPlaceholder("pattern").fill("?m OOC>");

  await dialog.locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("dialog", { name: "Praetor / Layout & display" }).locator(".entry-list button").filter({ hasText: "Arrange layout" }).click();
  dialog = page.getByRole("dialog", { name: "Layout", exact: true });
  const layoutBench = dialog.getByRole("list", { name: "View Bench" });
  const layoutSidebar2 = dialog.getByRole("group", { name: "Sidebar 2" });
  await dragAndDrop(
    page,
    layoutBench.getByRole("listitem").filter({ hasText: "Think / OOC" }),
    layoutSidebar2.locator(".destination"),
  );
  await layoutSidebar2.getByRole("spinbutton", { name: "Think / OOC height" }).fill("650");

  await dialog.locator(".mfoot").getByRole("button", { name: "Close", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await page.getByRole("dialog", { name: "Praetor / Layout & display" }).getByLabel("Close").click();
  await expect(page.getByTestId("e2e-toasts").locator(".toast")).toHaveCount(0, { timeout: 7_000 });

  await backend.text([
    "<Apula thinks aloud: I wonder...>",
    '<8:14 pm OOC> Constantine replies, "Indeed."',
    "A distant bell rings somewhere beyond the courtyard.",
  ]);

  const filtered = page.locator('aside[aria-label="Sidebar 2"]');
  await expect(filtered).toContainText("Think / OOC");
  await expect(filtered).toContainText("<Apula thinks aloud: I wonder...>");
  await expect(filtered).toContainText('<8:14 pm OOC> Constantine replies, "Indeed."');
  await expect(filtered).not.toContainText("A distant bell rings");
  await page.evaluate(() => document.fonts.ready);
  await mkdir(SCREENSHOT_DIR, { recursive: true });
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-layout-filtered-text-sidebar.png"),
    animations: "disabled",
    caret: "hide",
  });
});
