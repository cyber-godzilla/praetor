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

// resolveModeName maps a user-typed mode name to its canonical (case-correct)
// name, matching case-insensitively. "disable" is always valid. Returns null if
// no mode matches — callers use that to reject with a friendly message before a
// round-trip to the core (which also resolves case-insensitively).
export function resolveModeName(
  input: string,
  names: string[] | null | undefined,
): string | null {
  const lower = input.toLowerCase();
  if (lower === "disable") return "disable";
  for (const n of names ?? []) {
    if (n.toLowerCase() === lower) return n;
  }
  return null;
}
