// Numpad movement map. Convention: NumLock OFF = movement, NumLock ON = digits.
//
// WebKitGTK (the Linux webview) does not reliably report the NumLock lock-state
// through KeyboardEvent.getModifierState("NumLock") — it returns false — so we
// cannot use that to tell the two modes apart. Instead we read the mode from
// e.key, which IS engine-independent: with NumLock ON a numpad digit/decimal key
// reports its printable character ("8", "."); with NumLock OFF it reports a
// navigation value ("ArrowUp", "Delete", "Clear", "", "Unidentified", …). We move
// only in the latter case, keyed by the physical key (e.code).
//
// NumpadAdd/NumpadSubtract always report "+"/"-" regardless of NumLock, so they
// carry no NumLock signal. Since NumLock exists to let you type digits (and +/-
// aren't digits), they always drive up/down — the main-row +/- still type.
type NumpadEntry = {
  cmd: string;
  // The e.key value(s) this key reports when NumLock is ON (i.e. it's typing a
  // character, not navigating). Absent = no NumLock signal, so always move.
  printable?: string[];
};

const NUMPAD_COMMANDS: Record<string, NumpadEntry> = {
  Numpad7: { cmd: "nw", printable: ["7"] },
  Numpad8: { cmd: "n", printable: ["8"] },
  Numpad9: { cmd: "ne", printable: ["9"] },
  Numpad4: { cmd: "w", printable: ["4"] },
  Numpad5: { cmd: "look", printable: ["5"] },
  Numpad6: { cmd: "e", printable: ["6"] },
  Numpad1: { cmd: "sw", printable: ["1"] },
  Numpad2: { cmd: "s", printable: ["2"] },
  Numpad3: { cmd: "se", printable: ["3"] },
  Numpad0: { cmd: "ss", printable: ["0"] },
  NumpadDecimal: { cmd: "stand", printable: [".", ","] },
  NumpadSubtract: { cmd: "d" },
  NumpadAdd: { cmd: "u" },
};

// numpadCommand returns the game command for a numpad key press, or null when the
// key isn't a bound numpad key or NumLock is on (so the numpad types a digit and
// we leave the event alone). NumLock-on is inferred from `key` (e.key): a
// digit/decimal key whose value is its printable character.
export function numpadCommand(code: string, key: string): string | null {
  const entry = NUMPAD_COMMANDS[code];
  if (!entry) return null;
  if (entry.printable?.includes(key)) return null; // NumLock on → typing a digit
  return entry.cmd;
}
