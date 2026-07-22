import { describe, it, expect } from "vitest";
import { safeColor } from "./color";

describe("safeColor", () => {
  it("passes well-formed hex colors", () => {
    expect(safeColor("#fff")).toBe("#fff");
    expect(safeColor("#AABBCC")).toBe("#AABBCC");
    expect(safeColor("#12345678")).toBe("#12345678");
  });

  it("passes bare color keywords", () => {
    expect(safeColor("red")).toBe("red");
    expect(safeColor("white")).toBe("white");
  });

  it("fails closed on CSS injection attempts", () => {
    expect(safeColor("#0;position:fixed;inset:0;background:url(http://evil)")).toBe("");
    expect(safeColor("red;position:fixed")).toBe("");
    expect(safeColor("url(http://evil)")).toBe("");
    expect(safeColor("10px solid red")).toBe("");
    expect(safeColor("#12g")).toBe("");
  });

  it("treats empty/nullish as no color", () => {
    expect(safeColor("")).toBe("");
    expect(safeColor(undefined)).toBe("");
    expect(safeColor(null)).toBe("");
  });
});
