import { describe, it, expect } from "vitest";
import { clampMenuPosition } from "./menu";

describe("clampMenuPosition", () => {
  it("keeps an in-bounds menu where it is", () =>
    expect(clampMenuPosition(10, 10, 100, 60, 800, 600)).toEqual({ x: 10, y: 10 }));
  it("pulls a right-edge overflow back", () =>
    expect(clampMenuPosition(760, 10, 100, 60, 800, 600)).toEqual({ x: 700, y: 10 }));
  it("pulls a bottom-edge overflow up", () =>
    expect(clampMenuPosition(10, 580, 100, 60, 800, 600)).toEqual({ x: 10, y: 540 }));
  it("never goes negative", () =>
    expect(clampMenuPosition(-5, -5, 100, 60, 800, 600)).toEqual({ x: 0, y: 0 }));
});
