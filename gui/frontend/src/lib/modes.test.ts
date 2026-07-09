import { describe, it, expect } from "vitest";
import { modeList, isActiveMode } from "./modes";

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
