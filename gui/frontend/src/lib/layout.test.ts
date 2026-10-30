import { describe, expect, it } from "vitest";
import type { CustomTabConfig, DisplayProfileConfig } from "./types";
import {
  BUILTIN_VIEWS,
  COMPONENT_IDS,
  benchComponents,
  componentName,
  componentType,
  layoutComponentVisible,
  integerLayoutPixels,
  layoutWithIntegerPixels,
  collapsedSidebarIDs,
  deleteSidebar,
  mainComponentIDs,
  moveComponent,
  orderedSidebarIDs,
  responsiveExpandedSidebarIDs,
  sidebarComponentIDs,
} from "./layout";

const customID = "1fc95886-1541-4db4-8a49-d30391439c65";
const leftID = "e775577f-188c-4bdd-b0eb-071bfd347a39";
const rightID = "056998fb-35af-4f5d-a60b-dea9a847e77a";
const outerLeftID = "e35e85cb-5d23-4369-a751-a9bb1b318c58";

function placement(ComponentID: string, SidebarHeight = 240) {
  return { ComponentID, SidebarHeight, Collapsed: false };
}

function profile(): DisplayProfileConfig {
  return {
    ID: "0b40c7dd-fba7-44cb-83f7-f125f5bc98b3",
    Name: "Default",
    SidebarsVisible: true,
    Main: [placement(COMPONENT_IDS.all), placement(customID), placement(COMPONENT_IDS.metrics), placement(COMPONENT_IDS.debug)],
    Bench: [placement(COMPONENT_IDS.exits)],
    Sidebars: [
      { ID: leftID, Name: "Left", Enabled: true, Side: "left", Width: 200, Components: [placement(COMPONENT_IDS.vitals), placement(COMPONENT_IDS.map)] },
      { ID: rightID, Name: "Right", Enabled: true, Side: "right", Width: 240, Components: [placement(COMPONENT_IDS.actions)] },
      { ID: outerLeftID, Name: "Outer left", Enabled: true, Side: "left", Width: 280, Components: [placement(COMPONENT_IDS.modes)] },
    ],
  };
}

const custom: CustomTabConfig[] = [
  { ID: customID, Name: "Chat", Visible: true, EchoCommands: false, Rules: [] },
];

describe("UUID component placement", () => {
  it("rounds drag-resize pixels to bounded integers before persistence", () => {
    expect(integerLayoutPixels(259.6, 180, 600)).toBe(260);
    expect(integerLayoutPixels(79.7, 80, 800)).toBe(80);
    expect(integerLayoutPixels(800.8, 80, 800)).toBe(800);
    expect(Number.isInteger(integerLayoutPixels(233.49, 80, 800))).toBe(true);
  });

  it("normalizes every persisted layout dimension to an integer", () => {
    const value = profile();
    value.Main[0].SidebarHeight = 0.4;
    value.Bench[0].SidebarHeight = 201.7;
    value.Sidebars[0].Width = 259.6;
    value.Sidebars[0].Components[0].SidebarHeight = 333.3;

    const saved = layoutWithIntegerPixels(value);

    expect(saved.Main[0].SidebarHeight).toBe(0);
    expect(saved.Bench[0].SidebarHeight).toBe(202);
    expect(saved.Sidebars[0].Width).toBe(260);
    expect(saved.Sidebars[0].Components[0].SidebarHeight).toBe(333);
  });

  it("registers immutable UUIDs for every built-in component", () => {
    expect(BUILTIN_VIEWS.map((view) => view.kind)).toEqual([
      "all", "metrics", "debug", "map", "exits", "vitals", "actions", "modes", "variables",
    ]);
    expect(BUILTIN_VIEWS.every((view) => /^[0-9a-f-]{36}$/.test(view.id))).toBe(true);
    expect(new Set(BUILTIN_VIEWS.map((view) => view.id)).size).toBe(BUILTIN_VIEWS.length);
  });

  it("uses persisted list order independently in main and each sidebar", () => {
    const value = profile();
    expect(mainComponentIDs(value, custom, false)).toEqual([
      COMPONENT_IDS.all, customID, COMPONENT_IDS.metrics,
    ]);
    expect(sidebarComponentIDs(value.Sidebars[0], custom, false)).toEqual([
      COMPONENT_IDS.vitals, COMPONENT_IDS.map,
    ]);
  });

  it("only exposes the Debug component to the Layout editor in debug mode", () => {
    expect(layoutComponentVisible(COMPONENT_IDS.debug, false)).toBe(false);
    expect(layoutComponentVisible(COMPONENT_IDS.debug, true)).toBe(true);
    expect(layoutComponentVisible(COMPONENT_IDS.all, false)).toBe(true);
    expect(layoutComponentVisible(customID, false)).toBe(true);
  });

  it("orders same-side columns by profile order with the first closest to content", () => {
    const value = profile();
    expect(orderedSidebarIDs(value.Sidebars, "left")).toEqual([outerLeftID, leftID]);
    expect(orderedSidebarIDs(value.Sidebars, "right")).toEqual([rightID]);
  });

  it("supports more than three sidebars and collapses outer columns responsively", () => {
    const value = profile();
    value.Sidebars.push({
      ID: "f826f66e-9953-4802-8cde-bb6b8ea56738", Name: "Fourth", Enabled: true,
      Side: "right", Width: 260, Components: [placement(COMPONENT_IDS.variables)],
    });
    const ids = value.Sidebars.map((sidebar) => sidebar.ID);
    expect(responsiveExpandedSidebarIDs(value.Sidebars, ids, 1500, 480)).toEqual(ids);
    expect(responsiveExpandedSidebarIDs(value.Sidebars, ids, 1500, 480, [rightID])).toEqual(
      ids.filter((id) => id !== rightID),
    );
    expect(responsiveExpandedSidebarIDs(value.Sidebars, ids, 1000, 480)).toEqual(ids.slice(0, 2));
    expect(responsiveExpandedSidebarIDs(value.Sidebars, ids, 960, 480)).toEqual(ids.slice(0, 1));
  });

  it("identifies configured sidebars collapsed by the available workspace width", () => {
    expect(collapsedSidebarIDs([leftID, rightID, outerLeftID], [leftID])).toEqual([
      rightID,
      outerLeftID,
    ]);
    expect(collapsedSidebarIDs([leftID], [leftID])).toEqual([]);
  });

  it("drag moves one component by ID across lists and preserves its sidebar state", () => {
    const value = profile();
    value.Sidebars[0].Components[1].Collapsed = true;
    moveComponent(value, COMPONENT_IDS.map, { kind: "sidebar", sidebarID: rightID }, 0);
    expect(value.Sidebars[0].Components.map((item) => item.ComponentID)).toEqual([COMPONENT_IDS.vitals]);
    expect(value.Sidebars[1].Components.map((item) => item.ComponentID)).toEqual([COMPONENT_IDS.map, COMPONENT_IDS.actions]);
    expect(value.Sidebars[1].Components[0].Collapsed).toBe(true);
  });

  it("reorders within one destination using the previewed insertion index", () => {
    const value = profile();
    moveComponent(value, COMPONENT_IDS.all, { kind: "main" }, 2);
    expect(value.Main.map((item) => item.ComponentID)).toEqual([
      customID, COMPONENT_IDS.all, COMPONENT_IDS.metrics, COMPONENT_IDS.debug,
    ]);

    moveComponent(value, COMPONENT_IDS.debug, { kind: "main" }, 1);
    expect(value.Main.map((item) => item.ComponentID)).toEqual([
      customID, COMPONENT_IDS.debug, COMPONENT_IDS.all, COMPONENT_IDS.metrics,
    ]);
  });

  it("lists only unplaced components on the bench", () => {
    expect(benchComponents(profile(), custom)).toEqual([
      { id: COMPONENT_IDS.exits, name: "Compass" },
    ]);
  });

  it("never exposes an unresolved UUID as a component title", () => {
    expect(componentName(customID, custom)).toBe("Chat");
    expect(componentName("ad4cd04f-d4b3-470a-aa35-58dbd942732f", custom)).toBe("Unknown component");
  });

  it("identifies each component by its functional type without exposing identity", () => {
    expect(componentType(COMPONENT_IDS.all, custom)).toBe("Game output");
    expect(componentType(COMPONENT_IDS.map, custom)).toBe("Map");
    expect(componentType(COMPONENT_IDS.vitals, custom)).toBe("Status bars");
    expect(componentType(COMPONENT_IDS.actions, custom)).toBe("Action controls");
    expect(componentType(customID, custom)).toBe("Filtered text");
    expect(componentType("ad4cd04f-d4b3-470a-aa35-58dbd942732f", custom)).toBe("Unavailable");
  });

  it("deleting a sidebar moves its ordered components to the bench", () => {
    const value = profile();
    value.Sidebars[0].Components[1].Collapsed = true;
    deleteSidebar(value, leftID);
    expect(value.Sidebars.some((sidebar) => sidebar.ID === leftID)).toBe(false);
    expect(value.Main.map((item) => item.ComponentID)).toEqual([
      COMPONENT_IDS.all, customID, COMPONENT_IDS.metrics, COMPONENT_IDS.debug,
    ]);
    expect(value.Bench.map((item) => item.ComponentID)).toEqual([
      COMPONENT_IDS.exits, COMPONENT_IDS.vitals, COMPONENT_IDS.map,
    ]);
    expect(value.Bench[2].Collapsed).toBe(true);
  });
});
