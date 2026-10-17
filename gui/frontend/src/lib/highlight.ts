// Frontend rendering transforms that mirror internal/ui behavior: string
// highlighting (loot detection) and IP masking. These are visual-only and do
// not touch the core event stream.

import type { Segment, HighlightConfig } from "./types";
import { expandVariables } from "./variables";
import { compileWildcard, type WildcardPattern } from "./wildcard";

// Style -> {background, foreground} for the four highlight styles. Single
// source of truth, shared by the renderer and the highlights editor UI.
export const STYLE_COLORS: Record<string, { bg: string; fg: string }> = {
  red: { bg: "#cc4444", fg: "#ffffff" },
  gold: { bg: "#e8a838", fg: "#000000" },
  green: { bg: "#55cc55", fg: "#000000" },
  blue: { bg: "#88aaff", fg: "#000000" },
};

export interface CompiledHighlight {
  pattern: string;
  matcher?: WildcardPattern;
  bg: string;
  fg: string;
}

// Style for transient Ctrl+F search matches — deliberately distinct from the
// four configurable highlight styles so a search never masquerades as loot.
export const SEARCH_STYLE = { bg: "#7f5fb0", fg: "#ffffff" };

// foldAscii lowercases only ASCII A-Z, leaving every other code unit untouched.
// It is LENGTH-PRESERVING (unlike String.prototype.toLowerCase, which can grow a
// string — e.g. 'İ' → 'i̇'), so match offsets found in the folded string remain
// valid indexes into the original text. Mirrors Go's textutil.ToLowerASCII, which
// the highlight matcher relies on for exactly this reason. Highlighting is
// case-insensitive for ASCII only (matches the Go side).
export function foldAscii(s: string): string {
  let out = "";
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    out += c >= 65 && c <= 90 ? String.fromCharCode(c + 32) : s[i];
  }
  return out;
}

export function compileHighlights(
  rules: HighlightConfig[] | null | undefined,
  variables?: Record<string, string> | null,
): CompiledHighlight[] {
  return (rules ?? []).flatMap((r) => {
      if (!r.Active || !r.Pattern) return [];
      let pattern: string;
      try {
        pattern = expandVariables(r.Pattern, variables);
      } catch {
        return [];
      }
      if (!pattern) return [];
      const c = STYLE_COLORS[r.Style] ?? STYLE_COLORS.gold;
      return [{ pattern, matcher: compileWildcard(pattern, "substring"), bg: c.bg, fg: c.fg }];
    });
}

interface HSpan {
  start: number;
  end: number;
  fg: string;
  bg: string;
}

// applyHighlights matches highlight patterns against the WHOLE line first, then
// splits the segment list at match edges. Matching per-segment (as before) meant
// a pattern spanning a colorword/style boundary — "gold ring", where colorwords
// puts "gold" in its own segment — never matched, silently breaking the
// feature's core loot use case. Mirrors internal/ui/highlights.go.
export function applyHighlights(segments: Segment[], highlights: CompiledHighlight[]): Segment[] {
  if (highlights.length === 0) return segments;

  // Segment texts concatenate to the original line by construction.
  const text = segments.map((s) => s.text).join("");
  const spans = findHighlightSpans(text, highlights);
  if (spans.length === 0) return segments;

  const out: Segment[] = [];
  let pos = 0; // offset of the current segment's start within text
  for (const seg of segments) {
    const segStart = pos;
    const segEnd = pos + seg.text.length;
    pos = segEnd;
    if (seg.isHR || !seg.text) {
      out.push(seg);
      continue;
    }
    let cur = segStart;
    while (cur < segEnd) {
      const sp = nextSpanOverlapping(spans, cur, segEnd);
      if (!sp) {
        out.push({ ...seg, text: text.slice(cur, segEnd) });
        break;
      }
      if (sp.start > cur) {
        out.push({ ...seg, text: text.slice(cur, sp.start) });
        cur = sp.start;
      }
      const end = Math.min(sp.end, segEnd); // span may continue into the next segment
      out.push({ ...seg, text: text.slice(cur, end), color: sp.fg, bg: sp.bg });
      cur = end;
    }
  }
  return out;
}

// findHighlightSpans returns non-overlapping wildcard match spans over the
// original line. Earlier-configured patterns win overlaps.
function findHighlightSpans(text: string, highlights: CompiledHighlight[]): HSpan[] {
  const accepted: HSpan[] = [];
  for (const hl of highlights) {
    if (!hl.pattern) continue;
    const matches = hl.matcher
      ? hl.matcher.findAll(text)
      : literalSpans(text, hl.pattern);
    for (const { start, end } of matches) {
      if (!accepted.some((s) => start < s.end && end > s.start)) {
        accepted.push({ start, end, fg: hl.fg, bg: hl.bg });
      }
    }
  }
  accepted.sort((a, b) => a.start - b.start);
  return accepted;
}

function literalSpans(text: string, pattern: string): Array<{ start: number; end: number }> {
  const lower = foldAscii(text);
  const needle = foldAscii(pattern);
  const spans: Array<{ start: number; end: number }> = [];
  let offset = 0;
  for (;;) {
    const start = lower.indexOf(needle, offset);
    if (start < 0) return spans;
    const end = start + needle.length;
    spans.push({ start, end });
    offset = end;
  }
}

function nextSpanOverlapping(spans: HSpan[], lo: number, hi: number): HSpan | null {
  for (const s of spans) {
    if (s.end > lo && s.start < hi) return s;
  }
  return null;
}

// IP masking: replace dotted-quad IPs with a masked form, mirroring hide_ips.
// Matches any dotted-decimal sequence of 2+ octets; the replacer masks only
// true 4-octet quads, so a 5+-octet sequence (1.2.3.4.5) is left whole rather
// than having its first four octets partially masked into a real-looking string.
const IP_RE = /\b\d{1,3}(?:\.\d{1,3})+\b/g;

export function maskIPs(segments: Segment[]): Segment[] {
  return segments.map((seg) => {
    if (!seg.text) return seg;
    const masked = seg.text.replace(IP_RE, (m) => {
      const parts = m.split(".");
      if (parts.length !== 4) return m; // not an IPv4
      if (parts.some((p) => Number(p) > 255)) return m;
      return "***.***.***.***";
    });
    return masked === seg.text ? seg : { ...seg, text: masked };
  });
}
