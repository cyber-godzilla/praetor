import { readFileSync } from "node:fs";
import { mkdir } from "node:fs/promises";
import { basename, resolve } from "node:path";
import { test, expect } from "./test";
import { baseInit, makeConfig } from "./fixtures";
import type { Page } from "@playwright/test";
import type { ImagePayload, ModeSpec, RBResult, TextPayload, TrainingCostRow, WireEvent } from "../src/lib/types";

const SCREENSHOT_DIR = resolve("artifacts/ui-screenshots");
const GRAPHICS_DIR = resolve(SCREENSHOT_DIR, "graphics");

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
docsConfig.Commands.Variables = {
  count: "5",
  target: "bandit",
  weapon: "gladius",
};
docsConfig.Notifications.Desktop = {
  AllowScriptNotifications: true,
  Sound: false,
  HealthBelow: { Enabled: true, Threshold: 25 },
  FatigueBelow: { Enabled: false, Threshold: 10 },
  Patterns: [
    { Pattern: "retalq", Title: "Rare drop", Message: "Retalq appeared in the output.", Enabled: true },
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
  const regions = [
    { number: 1, target: page.locator(".statusbar"), badgeX: 0.66, badgeY: 0.5 },
    { number: 2, target: page.locator(".tabbar"), badgeX: 0.59, badgeY: 0.5 },
    { number: 3, target: page.locator(".output"), badgeX: 0.97, badgeY: 0.96 },
    { number: 4, target: page.getByRole("button", { name: "Map", exact: true }).locator(".."), badgeX: 0.94, badgeY: 0.08 },
    { number: 5, target: page.getByRole("button", { name: "Exits", exact: true }).locator(".."), badgeX: 0.94, badgeY: 0.08 },
    { number: 6, target: page.getByRole("button", { name: "Vitals", exact: true }).locator(".."), badgeX: 0.94, badgeY: 0.08 },
    { number: 7, target: page.locator(".sidebartabs"), badgeX: 0.94, badgeY: 0.06 },
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

  await expect(page.locator(".sidebar img")).toHaveCount(2);
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
  await expect(page.getByRole("dialog")).toContainText("Display & Behavior");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-menu.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByRole("dialog").getByRole("button", { name: "Settings", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText(";; chain delay (ms)");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-settings.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to menu").click();
  await page.getByRole("dialog").getByRole("button", { name: "Notifications", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText("Allow Script Notifications");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-notifications.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to menu").click();
  await page.getByRole("dialog").getByRole("button", { name: "Variables", exact: true }).click();
  await expect(page.getByLabel("Value for target")).toHaveValue("bandit");
  await page.screenshot({
    path: resolve(SCREENSHOT_DIR, "praetor-variables.png"),
    animations: "disabled",
    caret: "hide",
  });

  await page.getByTitle("Back to menu").click();
  await page.getByRole("dialog").getByRole("button", { name: "Rank-Bonus Calculator", exact: true }).click();
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

  await page.getByTitle("Back to menu").click();
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
