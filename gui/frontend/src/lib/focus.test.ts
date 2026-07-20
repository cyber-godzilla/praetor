import { describe, it, expect } from "vitest";
import { shouldRefocusInput, shouldRefocusFromClick, NON_REFOCUS_SELECTOR } from "./focus";

// Idle, unfocused, nothing selected: the sticky-focus should pull focus back.
const base = {
  modalOpen: false,
  pointerDown: false,
  selectionCollapsed: true,
  alreadyFocused: false,
  activeIsTextField: false,
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
  it("does not steal focus from another text field (search box)", () => {
    expect(shouldRefocusInput({ ...base, activeIsTextField: true })).toBe(false);
  });
});

const clickBase = {
  modalOpen: false,
  targetMatchesNonRefocus: false,
  selectionCollapsed: true,
};

describe("shouldRefocusFromClick", () => {
  it("refocuses when a non-text control (e.g. a button) is clicked", () => {
    expect(shouldRefocusFromClick(clickBase)).toBe(true);
  });
  it("does not refocus when a text field is clicked", () => {
    expect(shouldRefocusFromClick({ ...clickBase, targetMatchesNonRefocus: true })).toBe(false);
  });
  it("does not refocus while a modal is open", () => {
    expect(shouldRefocusFromClick({ ...clickBase, modalOpen: true })).toBe(false);
  });
  it("does not refocus while a text selection is live", () => {
    expect(shouldRefocusFromClick({ ...clickBase, selectionCollapsed: false })).toBe(false);
  });
  it("no longer excludes buttons or links from refocus", () => {
    expect(NON_REFOCUS_SELECTOR).not.toMatch(/\bbutton\b/);
    expect(NON_REFOCUS_SELECTOR).not.toMatch(/(^|[\s,])a([\s,]|$)/);
  });
});
