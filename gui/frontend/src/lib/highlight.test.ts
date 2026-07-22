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
    expect(match.bg).toBe("#cc4444");
    expect(match.color).toBe("#ffffff");
  });

  it("preserves original segment styling on non-matched runs", () => {
    const segs: Segment[] = [{ text: "a gem b", bold: true, color: "#123456" }];
    const out = applyHighlights(segs, compileHighlights(rules(["gem", "gold"])));
    expect(out[0]).toMatchObject({ text: "a ", bold: true, color: "#123456" });
    expect(out[1]).toMatchObject({ text: "gem", bold: true, bg: "#e8a838" });
    expect(out[2]).toMatchObject({ text: " b", bold: true, color: "#123456" });
  });

  it("matches a pattern spanning segment boundaries (colorword split)", () => {
    // Colorwords emits "gold" as its own styled segment; the loot pattern
    // "gold ring" must still match across the boundary.
    const segs: Segment[] = [
      { text: "gold", color: "#e8a838" },
      { text: " ring" },
    ];
    const out = applyHighlights(segs, compileHighlights(rules(["gold ring", "gold"])));
    const highlighted = out
      .filter((s) => s.bg === "#e8a838")
      .map((s) => s.text)
      .join("");
    expect(highlighted).toBe("gold ring");
    expect(out.map((s) => s.text).join("")).toBe("gold ring");
  });

  it("keeps highlight offsets correct with a length-changing lowercase char", () => {
    // 'İ' (U+0130) grows to two UTF-16 units under JS toLowerCase(); matching on
    // that would shift every following offset and tear the highlight. A
    // length-preserving fold keeps 'gold ring' landing exactly on 'gold ring'.
    const segs: Segment[] = [{ text: "İ a gold ring" }];
    const out = applyHighlights(segs, compileHighlights(rules(["gold ring", "gold"])));
    const highlighted = out
      .filter((s) => s.bg === "#e8a838")
      .map((s) => s.text)
      .join("");
    expect(highlighted).toBe("gold ring");
    expect(out.map((s) => s.text).join("")).toBe("İ a gold ring");
  });

  it("gives an earlier-configured pattern precedence on overlap", () => {
    const segs: Segment[] = [{ text: "gold ring" }];
    const out = applyHighlights(segs, compileHighlights(rules(["gold ring", "red"], ["gold", "green"])));
    // No run should carry the later "green" pattern's background.
    expect(out.some((s) => s.bg === "#55cc55")).toBe(false);
  });

  it("returns segments unchanged when no rules are active", () => {
    const segs: Segment[] = [{ text: "nothing here" }];
    expect(applyHighlights(segs, compileHighlights([]))).toBe(segs);
  });

  it("matches the earliest of multiple patterns", () => {
    const segs: Segment[] = [{ text: "silver and gold" }];
    const out = applyHighlights(segs, compileHighlights(rules(["gold", "gold"], ["silver", "blue"])));
    expect(out[0].text).toBe("silver");
    expect(out[0].bg).toBe("#88aaff");
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
  it("leaves 5-octet and out-of-range sequences unmasked", () => {
    expect(maskIPs([{ text: "1.2.3.4" }])[0].text).toBe("***.***.***.***");
    expect(maskIPs([{ text: "1.2.3.4.5" }])[0].text).toBe("1.2.3.4.5");
    expect(maskIPs([{ text: "999.1.1.1" }])[0].text).toBe("999.1.1.1");
    expect(maskIPs([{ text: "(1.2.3.4)" }])[0].text).toBe("(***.***.***.***)");
  });

  it("leaves non-IP text alone", () => {
    const seg: Segment = { text: "version 1.2.3 released" };
    expect(maskIPs([seg])[0].text).toBe("version 1.2.3 released");
  });
});
