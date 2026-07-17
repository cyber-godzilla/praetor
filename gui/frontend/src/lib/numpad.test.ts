import { describe, it, expect } from "vitest";
import { numpadCommand } from "./numpad";

describe("numpadCommand", () => {
  // NumLock OFF: the digit/decimal keys report navigation values in e.key, so
  // movement fires. (WebKitGTK doesn't report NumLock via getModifierState, so
  // we read state from e.key instead.)
  it("moves when a numpad key reports its NumLock-off navigation value", () => {
    const cases: [string, string, string][] = [
      ["Numpad7", "Home", "nw"],
      ["Numpad8", "ArrowUp", "n"],
      ["Numpad9", "PageUp", "ne"],
      ["Numpad4", "ArrowLeft", "w"],
      ["Numpad5", "Clear", "look"],
      ["Numpad6", "ArrowRight", "e"],
      ["Numpad1", "End", "sw"],
      ["Numpad2", "ArrowDown", "s"],
      ["Numpad3", "PageDown", "se"],
      ["Numpad0", "Insert", "ss"],
      ["NumpadDecimal", "Delete", "stand"],
    ];
    for (const [code, key, cmd] of cases) {
      expect(numpadCommand(code, key)).toBe(cmd);
    }
  });

  // Numpad5 can report "" or "Unidentified" (not "Clear") with NumLock off in
  // some engines — still not the printable "5", so it must still move.
  it("moves Numpad5 even when e.key is empty/unidentified", () => {
    expect(numpadCommand("Numpad5", "")).toBe("look");
    expect(numpadCommand("Numpad5", "Unidentified")).toBe("look");
  });

  // NumLock ON: the digit/decimal keys report their printable character, so we
  // leave the event alone and the numpad types normally.
  it("does not move when a digit/decimal key reports its printable character", () => {
    const printable: [string, string][] = [
      ["Numpad7", "7"], ["Numpad8", "8"], ["Numpad9", "9"],
      ["Numpad4", "4"], ["Numpad5", "5"], ["Numpad6", "6"],
      ["Numpad1", "1"], ["Numpad2", "2"], ["Numpad3", "3"],
      ["Numpad0", "0"], ["NumpadDecimal", "."], ["NumpadDecimal", ","],
    ];
    for (const [code, key] of printable) {
      expect(numpadCommand(code, key)).toBe(null);
    }
  });

  // NumpadAdd/Subtract always report "+"/"-" regardless of NumLock, so they
  // carry no NumLock signal and always drive up/down.
  it("always moves on numpad +/- regardless of e.key", () => {
    expect(numpadCommand("NumpadSubtract", "-")).toBe("d");
    expect(numpadCommand("NumpadAdd", "+")).toBe("u");
  });

  it("returns null for unmapped codes", () => {
    expect(numpadCommand("KeyA", "a")).toBe(null);
    expect(numpadCommand("NumpadEnter", "Enter")).toBe(null);
  });
});
