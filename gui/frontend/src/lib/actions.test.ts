import { describe, it, expect } from "vitest";
import { cycleIndex } from "./actions";

describe("cycleIndex", () => {
  it("advances forward", () => expect(cycleIndex(0, 1, 3)).toBe(1));
  it("wraps forward past the end", () => expect(cycleIndex(2, 1, 3)).toBe(0));
  it("wraps backward past the start", () => expect(cycleIndex(0, -1, 3)).toBe(2));
  it("stays put with a single item", () => expect(cycleIndex(0, 1, 1)).toBe(0));
  it("returns 0 for an empty list", () => expect(cycleIndex(0, -1, 0)).toBe(0));
});
