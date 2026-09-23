import { describe, expect, it } from "vitest";
import { normalizeDisplayMode, nextDisplayMode } from "./display";

describe("display mode", () => {
  it("toggles between sidebar and off", () => {
    expect(nextDisplayMode("sidebar")).toBe("off");
    expect(nextDisplayMode("off")).toBe("sidebar");
  });

  it("normalizes TUI-only and unknown persisted values to sidebar", () => {
    expect(normalizeDisplayMode("topbar")).toBe("sidebar");
    expect(normalizeDisplayMode("future-mode")).toBe("sidebar");
    expect(normalizeDisplayMode(undefined)).toBe("sidebar");
  });
});
