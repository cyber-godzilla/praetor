export type ShortcutArea = "game" | "output" | "input";

export interface ShortcutHelp {
  id: string;
  key: string;
  description: string;
  area: ShortcutArea;
}

// User-facing shortcut catalog. shortcuts.drift.test.ts scans the dispatch
// sites so a new app shortcut cannot silently go missing from Help.
export const SHORTCUTS: ShortcutHelp[] = [
  { id: "tab", key: "Tab / Shift+Tab", description: "Next / previous tab (Tab completes a visible command hint)", area: "game" },
  { id: "alt-digits", key: "Alt+1…9, Alt+0", description: "Jump to tab N", area: "game" },
  { id: "alt-s", key: "Alt+S", description: "Toggle sidebar", area: "game" },
  { id: "alt-m", key: "Alt+M", description: "Quick-cycle modes", area: "game" },
  { id: "alt-i", key: "Alt+I", description: "Show / hide suppressed lines", area: "game" },
  { id: "alt-x", key: "Alt+X", description: "Stop sends, queued chains, and scripts", area: "game" },
  { id: "ctrl-f", key: "Ctrl+F", description: "Search current scrollback", area: "game" },
  { id: "ctrl-r", key: "Ctrl+R", description: "Search command history", area: "game" },
  { id: "escape", key: "Esc", description: "Open menu or close the active layer", area: "game" },
  { id: "numpad", key: "Numpad", description: "Movement (according to navigation setting)", area: "game" },
  { id: "pageup", key: "PgUp", description: "Scroll output up one page", area: "output" },
  { id: "pagedown", key: "PgDn", description: "Scroll output down one page", area: "output" },
  { id: "home", key: "Home", description: "Scroll output to the top (or move within focused input)", area: "output" },
  { id: "end", key: "End", description: "Scroll output to the bottom (or move within focused input)", area: "output" },
  { id: "history", key: "↑ / ↓", description: "Previous / next command at an input boundary", area: "input" },
  { id: "enter", key: "Enter", description: "Send; modifiers insert a new line", area: "input" },
];
