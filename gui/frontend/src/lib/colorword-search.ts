import type { ColorwordStyle } from "./types";

export interface ColorwordStyleGroup {
  family: string;
  styles: ColorwordStyle[];
}

function normalized(value: string): string {
  return value.trim().toLocaleLowerCase();
}

function familyAliases(family: string): string[] {
  const name = normalized(family);
  return name.endsWith("s") ? [name, name.slice(0, -1)] : [name];
}

// A complete family name ("red" or "reds") expands to the whole family.
// Other text performs a live partial-name search ("crim" finds "Crimson").
export function searchColorwordStyles(
  styles: ColorwordStyle[],
  query: string,
): ColorwordStyle[] {
  const needle = normalized(query);
  if (!needle) return styles;

  const family = styles.find((style) => familyAliases(style.family).includes(needle))?.family;
  if (family) return styles.filter((style) => style.family === family);

  return styles
    .filter((style) =>
      normalized(style.label).includes(needle) || normalized(style.name).includes(needle),
    )
    .sort((left, right) => {
      const leftStarts = normalized(left.label).startsWith(needle);
      const rightStarts = normalized(right.label).startsWith(needle);
      if (leftStarts !== rightStarts) return leftStarts ? -1 : 1;
      return left.label.localeCompare(right.label);
    });
}

export function groupColorwordStyles(styles: ColorwordStyle[]): ColorwordStyleGroup[] {
  const groups = new Map<string, ColorwordStyle[]>();
  for (const style of styles) {
    const group = groups.get(style.family) ?? [];
    group.push(style);
    groups.set(style.family, group);
  }
  return [...groups].map(([family, groupStyles]) => ({ family, styles: groupStyles }));
}
