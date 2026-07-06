import { describe, it, expect } from "vitest";
import { applyHighlights, compileHighlights, maskIPs } from "./highlight";
import type { Segment, HighlightConfig } from "./types";

function rules(...r: [string, string][]): HighlightConfig[] {
  return r.map(([Pattern, Style]) => ({ Pattern, Style, Active: true }));
}

describe("applyHighlights", () => {
  it("splits a segment around a case-insensitive match", () => {
    const segs: Segment[] = [{ text: "you find a RUBY here" }];
    const hl = compileHighlights(rules(["ruby", "red"]));
    const out = applyHighlights(segs, hl);
    expect(out.map((s) => s.text)).toEqual(["you find a ", "RUBY", " here"]);
    const match = out[1];
    expect(match.bg).toBe("#e05c5c");
    expect(match.color).toBe("#ffffff");
  });

  it("preserves original segment styling on non-matched runs", () => {
    const segs: Segment[] = [{ text: "a gem b", bold: true, color: "#123456" }];
    const out = applyHighlights(segs, compileHighlights(rules(["gem", "gold"])));
    expect(out[0]).toMatchObject({ text: "a ", bold: true, color: "#123456" });
    expect(out[1]).toMatchObject({ text: "gem", bold: true, bg: "#e8a838" });
    expect(out[2]).toMatchObject({ text: " b", bold: true, color: "#123456" });
  });

  it("returns segments unchanged when no rules are active", () => {
    const segs: Segment[] = [{ text: "nothing here" }];
    expect(applyHighlights(segs, compileHighlights([]))).toBe(segs);
  });

  it("matches the earliest of multiple patterns", () => {
    const segs: Segment[] = [{ text: "silver and gold" }];
    const out = applyHighlights(segs, compileHighlights(rules(["gold", "gold"], ["silver", "blue"])));
    expect(out[0].text).toBe("silver");
    expect(out[0].bg).toBe("#5c8ce0");
  });

  it("ignores inactive rules", () => {
    const hl = compileHighlights([{ Pattern: "gold", Style: "gold", Active: false }]);
    expect(hl).toHaveLength(0);
  });

  it("leaves HR segments untouched", () => {
    const segs: Segment[] = [{ text: "", isHR: true }];
    const out = applyHighlights(segs, compileHighlights(rules(["x", "red"])));
    expect(out).toEqual(segs);
  });
});

describe("maskIPs", () => {
  it("masks dotted-quad addresses", () => {
    const out = maskIPs([{ text: "from 192.168.1.42 connected" }]);
    expect(out[0].text).toBe("from ***.***.***.*** connected");
  });
  it("leaves non-IP text alone", () => {
    const seg: Segment = { text: "version 1.2.3 released" };
    expect(maskIPs([seg])[0].text).toBe("version 1.2.3 released");
  });
});
