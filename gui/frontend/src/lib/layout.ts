import type {
  CustomTabConfig,
  DisplayComponentConfig,
  DisplayProfileConfig,
  DisplaySidebarConfig,
} from "./types";

export const COMPONENT_IDS = {
  all: "a1ebe82e-7f0d-4ba9-9e00-181d045e3e12",
  metrics: "3888f7f2-16fa-41df-86bc-11c7a2488dbf",
  debug: "a1ba3fe5-10e2-406f-b1f1-7f8746c89762",
  map: "ccb0ec1f-9a08-4d4f-941e-2b8b2c6eb8f1",
  exits: "85178cda-7db8-4fe1-a09d-1a6e572016b2",
  vitals: "e9a0240c-64df-4086-92aa-0f6372f8dde6",
  actions: "fbf29dda-9c5c-4457-8a8b-d5a106206c29",
  modes: "631a65d1-9261-4940-a18b-7ee7bfd48f12",
  variables: "c043ea23-cb1d-4a30-97bd-01e588969d26",
} as const;

export type BuiltinViewKind = keyof typeof COMPONENT_IDS;

export interface BuiltinViewDefinition {
  id: string;
  kind: BuiltinViewKind;
  name: string;
  type: string;
}

export const BUILTIN_VIEWS: BuiltinViewDefinition[] = [
  { id: COMPONENT_IDS.all, kind: "all", name: "All", type: "Game output" },
  { id: COMPONENT_IDS.metrics, kind: "metrics", name: "Metrics", type: "Session metrics" },
  { id: COMPONENT_IDS.debug, kind: "debug", name: "Debug", type: "Diagnostics" },
  { id: COMPONENT_IDS.map, kind: "map", name: "Map", type: "Map" },
  { id: COMPONENT_IDS.exits, kind: "exits", name: "Compass", type: "Compass" },
  { id: COMPONENT_IDS.vitals, kind: "vitals", name: "Vitals", type: "Status bars" },
  { id: COMPONENT_IDS.actions, kind: "actions", name: "Action Sets", type: "Action controls" },
  { id: COMPONENT_IDS.modes, kind: "modes", name: "Modes", type: "Automation modes" },
  { id: COMPONENT_IDS.variables, kind: "variables", name: "Variables", type: "Variable editor" },
];

export function newUUID(): string {
  const uuid = globalThis.crypto?.randomUUID?.();
  if (uuid) return uuid;
  // Tests and old embedded webviews without randomUUID still receive a UUID
  // shape; the backend validates and repairs it before persistence.
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (token) => {
    const value = Math.floor(Math.random() * 16);
    return (token === "x" ? value : (value & 0x3) | 0x8).toString(16);
  });
}

export const newCustomViewID = newUUID;

export function integerLayoutPixels(value: number, minimum: number, maximum: number): number {
  return Math.max(minimum, Math.min(maximum, Math.round(value)));
}

export function layoutWithIntegerPixels(profile: DisplayProfileConfig): DisplayProfileConfig {
  const integerPlacement = (placement: DisplayComponentConfig): DisplayComponentConfig => ({
    ...placement,
    SidebarHeight: Math.round(placement.SidebarHeight),
  });
  return {
    ...profile,
    Main: profile.Main.map(integerPlacement),
    Bench: profile.Bench.map(integerPlacement),
    Sidebars: profile.Sidebars.map((sidebar) => ({
      ...sidebar,
      Width: integerLayoutPixels(sidebar.Width, 180, 600),
      Components: sidebar.Components.map(integerPlacement),
    })),
  };
}

export function componentName(id: string, custom: CustomTabConfig[]): string {
  return BUILTIN_VIEWS.find((view) => view.id === id)?.name
    ?? custom.find((view) => view.ID === id)?.Name
    ?? "Unknown component";
}

export function componentType(id: string, custom: CustomTabConfig[]): string {
  const builtin = BUILTIN_VIEWS.find((view) => view.id === id);
  if (builtin) return builtin.type;
  return custom.some((view) => view.ID === id) ? "Filtered text" : "Unavailable";
}

export function componentKind(id: string): BuiltinViewKind | "custom" | undefined {
  return BUILTIN_VIEWS.find((view) => view.id === id)?.kind ?? (id ? "custom" : undefined);
}

export function layoutComponentVisible(id: string, debug: boolean): boolean {
  return id !== COMPONENT_IDS.debug || debug;
}

function enabledComponent(id: string, custom: CustomTabConfig[], debug: boolean): boolean {
  if (!layoutComponentVisible(id, debug)) return false;
  const configured = custom.find((view) => view.ID === id);
  return configured ? configured.Visible : true;
}

export function mainComponentIDs(profile: DisplayProfileConfig, custom: CustomTabConfig[], debug: boolean): string[] {
  return profile.Main
    .filter((component) => enabledComponent(component.ComponentID, custom, debug))
    .map((component) => component.ComponentID);
}

export function sidebarComponentIDs(sidebar: DisplaySidebarConfig, custom: CustomTabConfig[], debug: boolean): string[] {
  return sidebar.Components
    .filter((component) => enabledComponent(component.ComponentID, custom, debug))
    .map((component) => component.ComponentID);
}

export function orderedSidebarIDs(sidebars: DisplaySidebarConfig[], side: "left" | "right"): string[] {
  const ids = sidebars
    .filter((sidebar) => sidebar.Enabled && sidebar.Side === side)
    .map((sidebar) => sidebar.ID);
  return side === "left" ? ids.reverse() : ids;
}

export const COLLAPSED_SIDEBAR_WIDTH = 30;

export function responsiveExpandedSidebarIDs(
  sidebars: DisplaySidebarConfig[],
  nonemptyIDs: string[],
  availableWidth: number,
  minimumCentralWidth: number,
  manuallyCollapsedIDs: string[] = [],
  preferredExpandedID = "",
): string[] {
  const widths = new Map(sidebars.map((sidebar) => [sidebar.ID, sidebar.Width]));
  const configured = sidebars
    .filter((sidebar) => sidebar.Enabled && nonemptyIDs.includes(sidebar.ID))
    .map((sidebar) => sidebar.ID);
  const manuallyCollapsed = new Set(manuallyCollapsedIDs);
  const expanded = configured.filter((id) => !manuallyCollapsed.has(id));
  while (
    expanded.length > 0
    && configured.reduce(
      (sum, id) => sum + (expanded.includes(id) ? (widths.get(id) ?? 260) : COLLAPSED_SIDEBAR_WIDTH),
      minimumCentralWidth,
    ) > availableWidth
  ) {
    let collapseIndex = expanded.length - 1;
    while (collapseIndex >= 0 && expanded[collapseIndex] === preferredExpandedID) collapseIndex--;
    if (collapseIndex < 0) break;
    expanded.splice(collapseIndex, 1);
  }
  return expanded;
}

export function collapsedSidebarIDs(configuredIDs: string[], expandedIDs: string[]): string[] {
  const expanded = new Set(expandedIDs);
  return configuredIDs.filter((id) => !expanded.has(id));
}

export type ComponentDestination =
  | { kind: "main" }
  | { kind: "bench" }
  | { kind: "sidebar"; sidebarID: string };

export interface BenchComponent {
  id: string;
  name: string;
}

export function benchComponents(
  profile: DisplayProfileConfig,
  custom: CustomTabConfig[],
): BenchComponent[] {
  return (profile.Bench ?? []).map((component) => ({
    id: component.ComponentID,
    name: componentName(component.ComponentID, custom),
  }));
}

// moveComponent is the single ordering primitive used by drag/drop and the
// keyboard buttons. A component exists in exactly one visible destination or
// on the Bench.
export function moveComponent(
  profile: DisplayProfileConfig,
  componentID: string,
  destination: ComponentDestination,
  index: number,
): void {
  let placement: DisplayComponentConfig | undefined;
  let source: DisplayComponentConfig[] | undefined;
  let sourceIndex = -1;
  const remove = (items: DisplayComponentConfig[]) => {
    const found = items.findIndex((item) => item.ComponentID === componentID);
    if (found >= 0) {
      source = items;
      sourceIndex = found;
      placement = items.splice(found, 1)[0];
    }
  };
  remove(profile.Main);
  for (const sidebar of profile.Sidebars) remove(sidebar.Components);
  remove(profile.Bench);
  placement ??= { ComponentID: componentID, SidebarHeight: 300, Collapsed: false };

  const target = destination.kind === "main"
    ? profile.Main
    : destination.kind === "bench"
      ? profile.Bench
      : profile.Sidebars.find((sidebar) => sidebar.ID === destination.sidebarID)?.Components;
  if (!target) return;
  if (destination.kind === "main") {
    placement.SidebarHeight = 0;
    placement.Collapsed = false;
  } else if (placement.SidebarHeight < 80) {
    placement.SidebarHeight = 300;
  }
  const adjustedIndex = source === target && sourceIndex < index ? index - 1 : index;
  const position = Math.max(0, Math.min(adjustedIndex, target.length));
  target.splice(position, 0, placement);
}

export function deleteSidebar(profile: DisplayProfileConfig, sidebarID: string): void {
  const sidebar = profile.Sidebars.find((candidate) => candidate.ID === sidebarID);
  if (!sidebar) return;
  for (const component of [...sidebar.Components]) {
    moveComponent(profile, component.ComponentID, { kind: "bench" }, profile.Bench.length);
  }
  profile.Sidebars = profile.Sidebars.filter((candidate) => candidate.ID !== sidebarID);
}

export function newSidebar(index: number): DisplaySidebarConfig {
  return {
    ID: newUUID(),
    Name: `Sidebar ${index + 1}`,
    Enabled: true,
    Side: "right",
    Width: 260,
    Components: [],
  };
}
