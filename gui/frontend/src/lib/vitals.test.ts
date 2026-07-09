import { describe, it, expect } from "vitest";
import { vitalColor, vitalFillPct } from "./vitals";

describe("vitalColor", () => {
  it("dim for null", () => expect(vitalColor(null)).toBe("var(--fg-dim)"));
  it("green above 50", () => expect(vitalColor(51)).toBe("var(--green)"));
  it("orange at 50", () => expect(vitalColor(50)).toBe("var(--accent)"));
  it("orange at 26", () => expect(vitalColor(26)).toBe("var(--accent)"));
  it("red at 25", () => expect(vitalColor(25)).toBe("var(--red)"));
  it("red at 0", () => expect(vitalColor(0)).toBe("var(--red)"));
});

describe("vitalFillPct", () => {
  it("0 for null", () => expect(vitalFillPct(null)).toBe(0));
  it("clamps above 100", () => expect(vitalFillPct(150)).toBe(100));
  it("clamps below 0", () => expect(vitalFillPct(-5)).toBe(0));
  it("passes through mid values", () => expect(vitalFillPct(73)).toBe(73));
});
