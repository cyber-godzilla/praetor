// Decide whether the game-view input should reclaim focus. The input is
// "sticky" (it grabs focus so typing always works), but it must NOT steal focus
// while the user is selecting text in the output or has a live selection — doing
// so clears the selection and makes copy (Ctrl+C / right-click) impossible.
export function shouldRefocusInput(s: {
  modalOpen: boolean; // a modal owns the keyboard
  pointerDown: boolean; // a mouse gesture is in progress (likely selecting)
  selectionCollapsed: boolean; // false = a text selection is live
  alreadyFocused: boolean; // the input already has focus (no-op)
  activeIsTextField: boolean; // focus moved to another text field (e.g. the Ctrl+F search box) — leave it there
}): boolean {
  if (s.modalOpen) return false;
  if (s.pointerDown) return false;
  if (!s.selectionCollapsed) return false;
  if (s.alreadyFocused) return false;
  if (s.activeIsTextField) return false;
  return true;
}

// Clicks on these targets must NOT pull focus back to the command input:
// genuine text-entry fields (so the user can type in them) and the modal
// backdrop. Buttons and links are deliberately absent — clicking any non-text
// control returns focus to the input so the next keystroke types.
export const NON_REFOCUS_SELECTOR = "input, textarea, select, [contenteditable], .backdrop";

// Decide whether a click should return focus to the command input. The DOM glue
// resolves `targetMatchesNonRefocus` via el.closest(NON_REFOCUS_SELECTOR).
export function shouldRefocusFromClick(s: {
  modalOpen: boolean; // a modal owns the keyboard
  targetMatchesNonRefocus: boolean; // click landed on a text field or backdrop
  selectionCollapsed: boolean; // false = a text selection is live
}): boolean {
  if (s.modalOpen) return false;
  if (s.targetMatchesNonRefocus) return false;
  if (!s.selectionCollapsed) return false;
  return true;
}
