import { describe, expect, it } from "vitest";
import {
  lowercaseFirstCommandLetter,
  outerPageHasVerticalDrift,
  outputFontSizeForLayout,
  type OuterPageScrollMetrics,
} from "./mobile";

describe("responsive output font size", () => {
  it("keeps desktop and mobile values independent", () => {
    expect(outputFontSizeForLayout(14, 6, false)).toBe(14);
    expect(outputFontSizeForLayout(14, 6, true)).toBe(6);
  });
});

const pageAtTop: OuterPageScrollMetrics = {
  windowY: 0,
  scrollingElementTop: 0,
  documentElementTop: 0,
  bodyTop: 0,
  visualViewportOffsetTop: 0,
};

describe("mobile command normalization", () => {
  it.each([
    ["Look", "look"],
    ["  Attack thug", "  attack thug"],
    ["/Help", "/help"],
    ["2North", "2north"],
    ["🔥Écoute", "🔥écoute"],
  ])("lowercases only the first command letter in %j", (input, expected) => {
    expect(lowercaseFirstCommandLetter(input)).toBe(expected);
  });

  it.each([
    "look North",
    "/help Someone",
    "1234",
    "🔥 --",
    "",
  ])("leaves %j unchanged when its first letter is already lowercase or absent", (input) => {
    expect(lowercaseFirstCommandLetter(input)).toBe(input);
  });
});

describe("mobile outer-page scroll detection", () => {
  it("ignores an aligned page and sub-pixel viewport rounding", () => {
    expect(outerPageHasVerticalDrift(pageAtTop)).toBe(false);
    expect(
      outerPageHasVerticalDrift({
        ...pageAtTop,
        visualViewportOffsetTop: 0.75,
      }),
    ).toBe(false);
  });

  it.each<keyof OuterPageScrollMetrics>([
    "windowY",
    "scrollingElementTop",
    "documentElementTop",
    "bodyTop",
    "visualViewportOffsetTop",
  ])("detects vertical drift reported by %s", (metric) => {
    expect(
      outerPageHasVerticalDrift({
        ...pageAtTop,
        [metric]: 24,
      }),
    ).toBe(true);
  });
});
