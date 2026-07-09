// Mode-list helpers shared by the mode-select modal and the sidebar Modes tab.

// modeList prepends the always-available "disable" mode and de-duplicates, so a
// script that defines a mode literally named "disable" can't produce a duplicate
// key (which would crash a keyed {#each}).
export function modeList(names: string[] | null | undefined): string[] {
  return [...new Set(["disable", ...(names ?? [])])];
}

// isActiveMode reports whether `name` is the active mode. The engine reports an
// empty string for the default/disable mode, so "" and "disable" are equivalent.
export function isActiveMode(name: string, current: string): boolean {
  return current === name || (name === "disable" && (current === "" || current === "disable"));
}
