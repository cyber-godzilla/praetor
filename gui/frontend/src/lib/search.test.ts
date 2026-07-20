import { describe, it, expect } from "vitest";
import { matchingLineIds, stepIndex } from "./search";

const lines = [
  { id: 1, text: "A rat scurries past." },
  { id: 2, text: "You attack the RAT." },
  { id: 3, text: "The gate creaks open." },
  { id: 4, text: "" },
  { id: 5, text: "Ratface the beggar arrives." },
];

describe("matchingLineIds", () => {
  it("matches case-insensitively in buffer order", () => {
    expect(matchingLineIds(lines, "rat")).toEqual([1, 2, 5]);
  });
  it("matches substrings anywhere in the line", () => {
    expect(matchingLineIds(lines, "creaks")).toEqual([3]);
  });
  it("returns nothing for an empty or whitespace query", () => {
    expect(matchingLineIds(lines, "")).toEqual([]);
    expect(matchingLineIds(lines, "   ")).toEqual([]);
  });
  it("trims the query before matching", () => {
    expect(matchingLineIds(lines, "  gate ")).toEqual([3]);
  });
  it("returns nothing when nothing matches", () => {
    expect(matchingLineIds(lines, "dragon")).toEqual([]);
  });
});

describe("stepIndex", () => {
  it("returns -1 for an empty match list", () => {
    expect(stepIndex(0, 1, 0)).toBe(-1);
    expect(stepIndex(-1, -1, 0)).toBe(-1);
  });
  it("starts at the newest match when there is no selection", () => {
    expect(stepIndex(-1, -1, 5)).toBe(4);
    expect(stepIndex(-1, 1, 5)).toBe(4);
  });
  it("clamps an out-of-range index back to the newest match", () => {
    expect(stepIndex(9, -1, 5)).toBe(4);
  });
  it("steps backward (older) and wraps to the end", () => {
    expect(stepIndex(2, -1, 5)).toBe(1);
    expect(stepIndex(0, -1, 5)).toBe(4);
  });
  it("steps forward (newer) and wraps to the start", () => {
    expect(stepIndex(2, 1, 5)).toBe(3);
    expect(stepIndex(4, 1, 5)).toBe(0);
  });
});
