import { describe, expect, it } from "vitest";
import type { ColorwordStyle } from "./types";
import { groupColorwordStyles, searchColorwordStyles } from "./colorword-search";

const styles: ColorwordStyle[] = [
  { name: "scarlet", label: "Scarlet", family: "Reds", kind: "solid", colors: ["#f00"] },
  { name: "crimson", label: "Crimson", family: "Reds", kind: "solid", colors: ["#c00"] },
  { name: "cinnabar", label: "Cinnabar", family: "Oranges", kind: "solid", colors: ["#f60"] },
  { name: "rainbow", label: "Rainbow", family: "Special", kind: "rainbow", colors: ["#f00"] },
];

describe("colorword style search", () => {
  it("finds partial style names", () => {
    expect(searchColorwordStyles(styles, "crim").map((style) => style.name)).toEqual(["crimson"]);
  });

  it("expands singular and plural family names", () => {
    expect(searchColorwordStyles(styles, "red").map((style) => style.name)).toEqual([
      "scarlet",
      "crimson",
    ]);
    expect(searchColorwordStyles(styles, "reds").map((style) => style.name)).toEqual([
      "scarlet",
      "crimson",
    ]);
  });

  it("keeps the catalog's family order when grouping", () => {
    expect(groupColorwordStyles(styles).map((group) => group.family)).toEqual([
      "Reds",
      "Oranges",
      "Special",
    ]);
  });
});
