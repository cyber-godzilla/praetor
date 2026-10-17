import { describe, expect, it } from "vitest";
import type { CustomTabConfig, DisplayProfileConfig } from "./types";
import {
  BUILTIN_VIEWS,
  COMPONENT_IDS,
  mainComponentIDs,
  moveComponent,
  orderedSidebarIDs,
  responsiveSidebarIDs,
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

  it("orders same-side columns by profile order with the first closest to content", () => {
    const value = profile();
    expect(orderedSidebarIDs(value.Sidebars, "left")).toEqual([outerLeftID, leftID]);
    expect(orderedSidebarIDs(value.Sidebars, "right")).toEqual([rightID]);
  });

  it("supports more than three sidebars and sheds outer columns responsively", () => {
    const value = profile();
    value.Sidebars.push({
      ID: "f826f66e-9953-4802-8cde-bb6b8ea56738", Name: "Fourth", Enabled: true,
      Side: "right", Width: 260, Components: [placement(COMPONENT_IDS.variables)],
    });
    const ids = value.Sidebars.map((sidebar) => sidebar.ID);
    expect(responsiveSidebarIDs(value.Sidebars, ids, 1500, 480)).toEqual(ids);
    expect(responsiveSidebarIDs(value.Sidebars, ids, 1000, 480)).toEqual(ids.slice(0, 2));
  });

  it("drag moves one component by ID across lists and preserves its sidebar state", () => {
    const value = profile();
    value.Sidebars[0].Components[1].Collapsed = true;
    moveComponent(value, COMPONENT_IDS.map, { kind: "sidebar", sidebarID: rightID }, 0);
    expect(value.Sidebars[0].Components.map((item) => item.ComponentID)).toEqual([COMPONENT_IDS.vitals]);
    expect(value.Sidebars[1].Components.map((item) => item.ComponentID)).toEqual([COMPONENT_IDS.map, COMPONENT_IDS.actions]);
    expect(value.Sidebars[1].Components[0].Collapsed).toBe(true);
  });
});
