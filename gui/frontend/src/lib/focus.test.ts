import { describe, it, expect } from "vitest";
import { shouldRefocusInput } from "./focus";

// Idle, unfocused, nothing selected: the sticky-focus should pull focus back.
const base = {
  modalOpen: false,
  pointerDown: false,
  selectionCollapsed: true,
  alreadyFocused: false,
};

describe("shouldRefocusInput", () => {
  it("refocuses when idle and unfocused", () => {
    expect(shouldRefocusInput(base)).toBe(true);
  });
  it("does not refocus while a modal owns focus", () => {
    expect(shouldRefocusInput({ ...base, modalOpen: true })).toBe(false);
  });
  it("does not refocus mid mouse gesture (pointer down)", () => {
    expect(shouldRefocusInput({ ...base, pointerDown: true })).toBe(false);
  });
  it("does not refocus while a text selection is live", () => {
    expect(shouldRefocusInput({ ...base, selectionCollapsed: false })).toBe(false);
  });
  it("does not refocus when the input already has focus", () => {
    expect(shouldRefocusInput({ ...base, alreadyFocused: true })).toBe(false);
  });
});
