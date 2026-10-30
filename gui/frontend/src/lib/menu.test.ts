import { describe, it, expect } from "vitest";
import {
  MENU_SECTIONS,
  clampMenuPosition,
  groupMenuEntriesByScope,
  menuParent,
  menuScopeLabel,
  searchMenuEntries,
} from "./menu";

describe("clampMenuPosition", () => {
  it("keeps an in-bounds menu where it is", () =>
    expect(clampMenuPosition(10, 10, 100, 60, 800, 600)).toEqual({ x: 10, y: 10 }));
  it("pulls a right-edge overflow back", () =>
    expect(clampMenuPosition(760, 10, 100, 60, 800, 600)).toEqual({ x: 700, y: 10 }));
  it("pulls a bottom-edge overflow up", () =>
    expect(clampMenuPosition(10, 580, 100, 60, 800, 600)).toEqual({ x: 10, y: 540 }));
  it("never goes negative", () =>
    expect(clampMenuPosition(-5, -5, 100, 60, 800, 600)).toEqual({ x: 0, y: 0 }));
});

describe("Escape menu architecture", () => {
  it("keeps the top-level settings areas in their intended order", () => {
    expect(MENU_SECTIONS.map((section) => section.label)).toEqual([
      "Character",
      "Layout & display",
      "Input & commands",
      "Automation",
      "Notifications & logs",
      "Themes",
      "Tools & help",
      "Application",
    ]);
  });

  it("groups a category by actual setting scope", () => {
    const display = MENU_SECTIONS.find((section) => section.id === "display")!;
    const groups = groupMenuEntriesByScope(display);
    expect(groups.map((group) => group.scope)).toEqual(["character", "global"]);
    expect(groups.find((group) => group.scope === "global")?.entries.map((entry) => entry.label))
      .toContain("View Library");
    expect(groups.find((group) => group.scope === "character")?.entries.map((entry) => entry.label))
      .toContain("Highlights");
  });

  it("searches labels, explanations, and section names", () => {
    expect(searchMenuEntries("speech").map((result) => result.entry.label)).toEqual(["Speech colors"]);
    expect(searchMenuEntries("startup").map((result) => result.entry.label)).toContain("Application settings");
    expect(searchMenuEntries("theme").some((result) => result.section.id === "themes")).toBe(true);
    expect(searchMenuEntries("   ")).toEqual([]);
  });

  it("gives every editor a stable parent instead of returning everything to the root", () => {
    expect(menuParent("menu-display")).toBe("menu");
    expect(menuParent("layout")).toBe("menu-display");
    expect(menuParent("variables")).toBe("menu-input");
    expect(menuParent("scripts")).toBe("menu-automation");
    expect(menuParent("notifications")).toBe("menu-notifications");
    expect(menuParent("help")).toBe("menu-tools");
    expect(menuParent("new-user")).toBe("menu-application");
    expect(menuParent("modeselect")).toBe("menu-automation");
    expect(menuParent("wiki")).toBe("menu-tools");
    expect(menuParent("maps")).toBe("menu-tools");
    expect(menuParent("calc")).toBe("menu-tools");
    expect(menuParent("kudos")).toBe("menu-tools");
    expect(menuParent("notes")).toBe("menu-tools");
  });

  it("labels setting scope in player language", () => {
    expect(menuScopeLabel("character")).toBe("Current character");
    expect(menuScopeLabel("global")).toBe("All characters");
    expect(menuScopeLabel("device")).toBe("This device");
    expect(menuScopeLabel("server")).toBe("Praetor server");
  });
});
