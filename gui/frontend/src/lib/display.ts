// topbar remains a valid shared config value for the TUI, but the GUI only has
// enough horizontal structure for a sidebar or an unobstructed output pane.
export type DisplayMode = "sidebar" | "off";

export function normalizeDisplayMode(value: string | null | undefined): DisplayMode {
  if (value === "off") return value;
  return "sidebar";
}

export function nextDisplayMode(mode: DisplayMode): DisplayMode {
  return mode === "sidebar" ? "off" : "sidebar";
}
