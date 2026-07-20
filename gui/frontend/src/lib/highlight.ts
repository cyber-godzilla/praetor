// Frontend rendering transforms that mirror internal/ui behavior: string
// highlighting (loot detection) and IP masking. These are visual-only and do
// not touch the core event stream.

import type { Segment, HighlightConfig } from "./types";

// Style -> {background, foreground} for the four highlight styles. Single
// source of truth, shared by the renderer and the highlights editor UI.
export const STYLE_COLORS: Record<string, { bg: string; fg: string }> = {
  red: { bg: "#cc4444", fg: "#ffffff" },
  gold: { bg: "#e8a838", fg: "#000000" },
  green: { bg: "#55cc55", fg: "#000000" },
  blue: { bg: "#88aaff", fg: "#000000" },
};

export interface CompiledHighlight {
  pattern: string; // lowercased
  bg: string;
  fg: string;
}

// Style for transient Ctrl+F search matches — deliberately distinct from the
// four configurable highlight styles so a search never masquerades as loot.
export const SEARCH_STYLE = { bg: "#7f5fb0", fg: "#ffffff" };

export function compileHighlights(rules: HighlightConfig[] | null | undefined): CompiledHighlight[] {
  return (rules ?? [])
    .filter((r) => r.Active && r.Pattern)
    .map((r) => {
      const c = STYLE_COLORS[r.Style] ?? STYLE_COLORS.gold;
      return { pattern: r.Pattern.toLowerCase(), bg: c.bg, fg: c.fg };
    });
}

// applyHighlights splits each segment at case-insensitive substring matches,
// tagging matched runs with the highlight background/foreground. Non-matched
// text keeps its original styling. Returns a new segment array.
export function applyHighlights(segments: Segment[], highlights: CompiledHighlight[]): Segment[] {
  if (highlights.length === 0) return segments;
  const out: Segment[] = [];
  for (const seg of segments) {
    if (seg.isHR || !seg.text) {
      out.push(seg);
      continue;
    }
    out.push(...splitSegment(seg, highlights));
  }
  return out;
}

function splitSegment(seg: Segment, highlights: CompiledHighlight[]): Segment[] {
  const text = seg.text;
  const lower = text.toLowerCase();
  const result: Segment[] = [];
  let i = 0;
  while (i < text.length) {
    // Find the earliest match at or after i across all patterns.
    let best = -1;
    let bestLen = 0;
    let bestHl: CompiledHighlight | null = null;
    for (const hl of highlights) {
      const idx = lower.indexOf(hl.pattern, i);
      if (idx !== -1 && (best === -1 || idx < best)) {
        best = idx;
        bestLen = hl.pattern.length;
        bestHl = hl;
      }
    }
    if (best === -1 || !bestHl) {
      result.push({ ...seg, text: text.slice(i) });
      break;
    }
    if (best > i) {
      result.push({ ...seg, text: text.slice(i, best) });
    }
    result.push({
      ...seg,
      text: text.slice(best, best + bestLen),
      color: bestHl.fg,
      bg: bestHl.bg,
    });
    i = best + bestLen;
  }
  return result;
}

// IP masking: replace dotted-quad IPs with a masked form, mirroring hide_ips.
const IP_RE = /\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b/g;

export function maskIPs(segments: Segment[]): Segment[] {
  return segments.map((seg) =>
    seg.text && IP_RE.test(seg.text)
      ? { ...seg, text: seg.text.replace(IP_RE, "***.***.***.***") }
      : seg,
  );
}
