export type MenuScope = "character" | "global" | "device" | "server";

export interface MenuEntry {
  label: string;
  description: string;
  route?: string;
  action?: "reload-scripts";
  scope?: MenuScope;
}

export interface MenuSection {
  id: string;
  label: string;
  description: string;
  scope: MenuScope;
  entries: MenuEntry[];
}

// This is the single inventory for the Escape menu. Keep editors here rather
// than scattering links through component markup so a setting cannot quietly
// disappear when the navigation changes again.
export const MENU_SECTIONS: MenuSection[] = [
  {
    id: "character",
    label: "Character",
    description: "Profiles and character switching",
    scope: "character",
    entries: [
      {
        label: "Manage character profiles",
        description: "Add, rename, number, and remove character profiles.",
        route: "character-profiles",
        scope: "global",
      },
    ],
  },
  {
    id: "display",
    label: "Layout & display",
    description: "Views, text, filtering, and mobile",
    scope: "character",
    entries: [
      { label: "Arrange layout", description: "Place views in tabs, sidebars, or the Bench.", route: "layout" },
      { label: "View Library", description: "Browse every character's reusable views and see where each is used.", route: "view-library", scope: "global" },
      { label: "Text & display settings", description: "Colorwords, text size, wounds, map, compass, and mobile display.", route: "settings-display", scope: "global" },
      { label: "Highlights", description: "Color matching game output for the current character.", route: "highlights", scope: "character" },
      { label: "Speech colors", description: "Assign Colorword styles to character speech.", route: "speech-colors", scope: "global" },
      { label: "Ignored OOC accounts", description: "Suppress selected OOC speakers.", route: "ignore-ooc", scope: "global" },
      { label: "Ignored Think characters", description: "Suppress selected Think speakers.", route: "ignore-think", scope: "global" },
    ],
  },
  {
    id: "input",
    label: "Input & commands",
    description: "Command entry, variables, and action sets",
    scope: "character",
    entries: [
      { label: "Input behavior", description: "Spellcheck, command echo, delays, and numpad navigation.", route: "settings-input", scope: "global" },
      { label: "Variables", description: "Reusable values for commands and action sets.", route: "variables" },
      { label: "Action sets", description: "Create and arrange quick command buttons.", route: "actionsets" },
      { label: "High-priority commands", description: "Commands allowed to bypass automation queues.", route: "priority" },
    ],
  },
  {
    id: "automation",
    label: "Automation",
    description: "Modes, scripts, and long-term data",
    scope: "character",
    entries: [
      { label: "Switch mode", description: "Select the active Lua mode.", route: "modeselect" },
      { label: "Quick-cycle modes", description: "Choose modes available through Alt+M.", route: "quickcycle" },
      { label: "Script Manager", description: "Manage sources, updates, trust, and local directories.", route: "scripts", scope: "global" },
      { label: "Reload scripts", description: "Reload modes and command hints from disk.", action: "reload-scripts", scope: "global" },
      { label: "Long-term script data", description: "Inspect accumulated per-character records.", route: "persistent" },
    ],
  },
  {
    id: "notifications",
    label: "Notifications & logs",
    description: "Alerts, delivery, and records",
    scope: "global",
    entries: [
      { label: "Notifications", description: "Configure alert rules and delivery channels.", route: "notifications" },
      { label: "Logging", description: "Session transcripts, paths, and application log retention.", route: "settings-logs", scope: "server" },
    ],
  },
  {
    id: "themes",
    label: "Themes",
    description: "Appearance presets and visual effects",
    scope: "device",
    entries: [
      { label: "Retro CRT effects", description: "Configure the optional CRT presentation.", route: "crt" },
    ],
  },
  {
    id: "tools",
    label: "Tools & help",
    description: "Notes, references, and utilities",
    scope: "global",
    entries: [
      { label: "Notes", description: "Write and organize personal notes.", route: "notes" },
      { label: "Kudos", description: "Manage favorites and queued kudos.", route: "kudos" },
      { label: "Rank-bonus calculator", description: "Compare ranks and training costs.", route: "calc" },
      { label: "Wiki bookmarks", description: "Open saved TEC Wiki references.", route: "wiki" },
      { label: "Map bookmarks", description: "Open saved game maps.", route: "maps" },
      { label: "Help", description: "Commands, shortcuts, syntax, and game help.", route: "help" },
    ],
  },
  {
    id: "application",
    label: "Application",
    description: "Privacy, updates, accounts, and integration",
    scope: "device",
    entries: [
      { label: "Application settings", description: "Privacy and startup update checks.", route: "settings-application" },
      { label: "Getting started", description: "Review the first-run setup guide.", route: "new-user" },
    ],
  },
];

const parents = new Map<string, string>();
for (const section of MENU_SECTIONS) {
  const sectionRoute = `menu-${section.id}`;
  parents.set(sectionRoute, "menu");
  for (const entry of section.entries) {
    if (entry.route) parents.set(entry.route, sectionRoute);
  }
}

export function menuSection(id: string): MenuSection | undefined {
  return MENU_SECTIONS.find((section) => section.id === id);
}

export function menuParent(route: string | null | undefined): string | null {
  if (!route) return null;
  return parents.get(route) ?? null;
}

export function menuRouteLabel(route: string | null | undefined): string {
  if (route === "menu") return "Menu";
  if (route?.startsWith("menu-")) return menuSection(route.slice(5))?.label ?? "Menu";
  for (const section of MENU_SECTIONS) {
    const entry = section.entries.find((candidate) => candidate.route === route);
    if (entry) return entry.label;
  }
  return "Menu";
}

export function menuScopeLabel(scope: MenuScope): string {
  switch (scope) {
    case "character": return "Current character";
    case "global": return "All characters";
    case "device": return "This device";
    case "server": return "Praetor server";
  }
}

const SCOPE_ORDER: MenuScope[] = ["character", "global", "device", "server"];

export function groupMenuEntriesByScope(
  section: MenuSection,
  resolveScope: (entry: MenuEntry) => MenuScope = (entry) => entry.scope ?? section.scope,
): { scope: MenuScope; entries: MenuEntry[] }[] {
  return SCOPE_ORDER
    .map((scope) => ({
      scope,
      entries: section.entries.filter((entry) => resolveScope(entry) === scope),
    }))
    .filter((group) => group.entries.length > 0);
}

export interface MenuSearchResult {
  section: MenuSection;
  entry: MenuEntry;
}

export function searchMenuEntries(query: string): MenuSearchResult[] {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return [];
  const results: MenuSearchResult[] = [];
  for (const section of MENU_SECTIONS) {
    for (const entry of section.entries) {
      const haystack = `${section.label} ${section.description} ${entry.label} ${entry.description}`.toLocaleLowerCase();
      if (haystack.includes(needle)) results.push({ section, entry });
    }
  }
  return results;
}

// Clamp a context-menu's top-left so the whole menu stays within the viewport.
export function clampMenuPosition(
  x: number,
  y: number,
  menuW: number,
  menuH: number,
  viewW: number,
  viewH: number,
): { x: number; y: number } {
  return {
    x: Math.max(0, Math.min(x, viewW - menuW)),
    y: Math.max(0, Math.min(y, viewH - menuH)),
  };
}
