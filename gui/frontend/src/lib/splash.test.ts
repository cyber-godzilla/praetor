import { describe, it, expect } from "vitest";
import { centerInBox } from "./splash";

describe("centerInBox", () => {
  // Regression: the version was hard-truncated to 6 chars, so "v0.2.10" (7
  // chars, the first two-digit patch) rendered as "v0.2.1" on the splash.
  it("keeps the full version and fills the exact box width", () => {
    const r = centerInBox("v0.2.10", 64);
    expect(r.length).toBe(64);
    expect(r.trim()).toBe("v0.2.10");
  });

  it("centers the version within the field", () => {
    const r = centerInBox("v0.2.10", 64);
    const lead = r.length - r.trimStart().length;
    expect(lead).toBe(Math.floor((64 - "v0.2.10".length) / 2)); // 28
  });

  it("previous ≤6-char versions still fill the width", () => {
    for (const v of ["dev", "v0.2.9"]) {
      const r = centerInBox(v, 64);
      expect(r.length).toBe(64);
      expect(r.trim()).toBe(v);
    }
  });

  it("clips an absurdly long version so the border never overflows", () => {
    expect(centerInBox("x".repeat(100), 64).length).toBe(64);
  });
});
