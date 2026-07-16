// Numpad movement map. The convention is NumLock OFF = movement (so with NumLock
// ON the numpad still types digits normally). Keyed by e.code (physical key), so
// it is independent of the composed e.key the browser reports.
const NUMPAD_COMMANDS: Record<string, string> = {
  Numpad7: "nw",
  Numpad8: "n",
  Numpad9: "ne",
  Numpad4: "w",
  Numpad5: "look",
  Numpad6: "e",
  Numpad1: "sw",
  Numpad2: "s",
  Numpad3: "se",
  Numpad0: "ss",
  NumpadDecimal: "stand",
  NumpadSubtract: "d",
  NumpadAdd: "u",
};

// numpadCommand returns the game command for a numpad key press, or null if the
// key isn't a bound numpad key or NumLock is on (in which case the numpad types
// digits and we leave the event alone).
export function numpadCommand(code: string, numLockOn: boolean): string | null {
  if (numLockOn) return null;
  return NUMPAD_COMMANDS[code] ?? null;
}
