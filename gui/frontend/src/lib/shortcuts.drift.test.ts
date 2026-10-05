import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { SHORTCUTS, type ShortcutArea } from "./shortcuts";

const read = (rel: string) => readFileSync(new URL(rel, import.meta.url), "utf8");

function catalog(area: ShortcutArea): Set<string> {
  return new Set(SHORTCUTS.filter((s) => s.area === area).map((s) => s.id));
}

function gameShortcuts(): Set<string> {
  const src = read("../components/GameView.svelte");
  const found = new Set<string>();
  if (src.includes('e.key === "Escape"')) found.add("escape");
  if (src.includes('e.code === "Tab"')) found.add("tab");
  if (src.includes("numpadCommand(")) found.add("numpad");
  if (src.includes('e.code.match(/^Digit')) found.add("alt-digits");
  const ctrl = src.slice(src.indexOf("// Ctrl+F opens"), src.indexOf("// Numpad navigation"));
  for (const key of ctrl.matchAll(/e\.code === "Key([A-Z])"/g)) {
    found.add(`ctrl-${key[1].toLowerCase()}`);
  }
  const alt = src.slice(src.indexOf("if (e.altKey) {"), src.indexOf("const activeTab"));
  for (const key of alt.matchAll(/e\.code === "Key([A-Z])"/g)) {
    found.add(`alt-${key[1].toLowerCase()}`);
  }
  return found;
}

function outputShortcuts(): Set<string> {
  const src = read("../components/OutputPane.svelte");
  const ids = new Set<string>();
  for (const key of src.matchAll(/case "(PageUp|PageDown|Home|End)"/g)) {
    ids.add(key[1].toLowerCase());
  }
  return ids;
}

function inputShortcuts(): Set<string> {
  const src = read("../components/InputLine.svelte");
  const ids = new Set<string>();
  if (src.includes('e.key === "ArrowUp"') && src.includes('e.key === "ArrowDown"')) ids.add("history");
  if (src.includes("function submit(")) ids.add("enter");
  return ids;
}

describe("Help shortcut catalog stays aligned with dispatch", () => {
  it.each([
    ["game", gameShortcuts],
    ["output", outputShortcuts],
    ["input", inputShortcuts],
  ] as const)("matches the %s shortcuts exactly", (area, scan) => {
    expect([...scan()].sort()).toEqual([...catalog(area)].sort());
  });
});
