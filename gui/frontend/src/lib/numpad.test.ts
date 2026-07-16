import { describe, it, expect } from "vitest";
import { numpadCommand } from "./numpad";

describe("numpadCommand", () => {
  it("maps every numpad key to its command when NumLock is off", () => {
    const cases: [string, string][] = [
      ["Numpad7", "nw"], ["Numpad8", "n"], ["Numpad9", "ne"],
      ["Numpad4", "w"], ["Numpad5", "look"], ["Numpad6", "e"],
      ["Numpad1", "sw"], ["Numpad2", "s"], ["Numpad3", "se"],
      ["Numpad0", "ss"], ["NumpadDecimal", "stand"],
      ["NumpadSubtract", "d"], ["NumpadAdd", "u"],
    ];
    for (const [code, cmd] of cases) {
      expect(numpadCommand(code, false)).toBe(cmd);
    }
  });
  it("returns null for every numpad key when NumLock is on", () => {
    expect(numpadCommand("Numpad8", true)).toBe(null);
    expect(numpadCommand("NumpadAdd", true)).toBe(null);
  });
  it("returns null for unmapped codes", () => {
    expect(numpadCommand("KeyA", false)).toBe(null);
    expect(numpadCommand("NumpadEnter", false)).toBe(null);
  });
});
