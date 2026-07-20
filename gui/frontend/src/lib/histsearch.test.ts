import { describe, it, expect } from "vitest";
import { searchBackward, dropLastChar } from "./histsearch";

const history = ["look", "attack rat", "get all from ground", "Attack bear", "say hello"];

describe("searchBackward", () => {
  it("finds the newest match at or before `from`", () => {
    expect(searchBackward(history, "attack", history.length - 1)).toBe(3);
  });
  it("continues to older matches from an earlier position", () => {
    expect(searchBackward(history, "attack", 2)).toBe(1);
  });
  it("matches case-insensitively", () => {
    expect(searchBackward(history, "ATTACK", 2)).toBe(1);
  });
  it("returns -1 when no older match exists", () => {
    expect(searchBackward(history, "attack", 0)).toBe(-1);
  });
  it("returns -1 for an empty query", () => {
    expect(searchBackward(history, "", history.length - 1)).toBe(-1);
  });
  it("clamps `from` beyond the end to the newest entry", () => {
    expect(searchBackward(history, "hello", 99)).toBe(4);
  });
  it("returns -1 on empty history", () => {
    expect(searchBackward([], "x", 0)).toBe(-1);
  });
});

describe("dropLastChar", () => {
  it("drops a plain ASCII character", () => {
    expect(dropLastChar("abc")).toBe("ab");
  });
  it("drops a full surrogate pair, not half of it", () => {
    expect(dropLastChar("hi😀")).toBe("hi");
  });
  it("handles the empty string", () => {
    expect(dropLastChar("")).toBe("");
  });
});
