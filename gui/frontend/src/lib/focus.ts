// Decide whether the game-view input should reclaim focus. The input is
// "sticky" (it grabs focus so typing always works), but it must NOT steal focus
// while the user is selecting text in the output or has a live selection — doing
// so clears the selection and makes copy (Ctrl+C / right-click) impossible.
export function shouldRefocusInput(s: {
  modalOpen: boolean; // a modal owns the keyboard
  pointerDown: boolean; // a mouse gesture is in progress (likely selecting)
  selectionCollapsed: boolean; // false = a text selection is live
  alreadyFocused: boolean; // the input already has focus (no-op)
}): boolean {
  if (s.modalOpen) return false;
  if (s.pointerDown) return false;
  if (!s.selectionCollapsed) return false;
  if (s.alreadyFocused) return false;
  return true;
}
