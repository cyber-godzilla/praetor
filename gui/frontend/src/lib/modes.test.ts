import { describe, it, expect } from "vitest";
import { modeList, isActiveMode, resolveModeName } from "./modes";

describe("modeList", () => {
  it("prepends disable", () => expect(modeList(["combat"])).toEqual(["disable", "combat"]));
  it("dedupes a script-defined disable", () =>
    expect(modeList(["disable", "combat"])).toEqual(["disable", "combat"]));
  it("handles null", () => expect(modeList(null)).toEqual(["disable"]));
  it("handles undefined", () => expect(modeList(undefined)).toEqual(["disable"]));
});

describe("isActiveMode", () => {
  it("matches exact name", () => expect(isActiveMode("combat", "combat")).toBe(true));
  it("empty current means disable", () => expect(isActiveMode("disable", "")).toBe(true));
  it("disable current means disable", () => expect(isActiveMode("disable", "disable")).toBe(true));
  it("non-match is false", () => expect(isActiveMode("combat", "craft")).toBe(false));
  it("disable is not active under a real mode", () => expect(isActiveMode("disable", "combat")).toBe(false));
});

describe("resolveModeName", () => {
  const names = ["Fishing", "combat"];
  it("returns the canonical name for a case-insensitive match", () => {
    expect(resolveModeName("fishing", names)).toBe("Fishing");
    expect(resolveModeName("COMBAT", names)).toBe("combat");
  });
  it("returns an exact match unchanged", () => {
    expect(resolveModeName("Fishing", names)).toBe("Fishing");
  });
  it("always resolves disable regardless of case or list", () => {
    expect(resolveModeName("DISABLE", null)).toBe("disable");
  });
  it("returns null for an unknown mode", () => {
    expect(resolveModeName("nope", names)).toBe(null);
  });
});
