// Shared vital-bar helpers for the top StatusBar and the sidebar vitals gauges.
// Kept in one place so the color thresholds never drift between the two views.

// vitalColor mirrors internal/ui/statusbar.go: >50 green, >25 orange, else red.
export function vitalColor(v: number | null): string {
  if (v == null) return "var(--fg-dim)";
  if (v > 50) return "var(--green)";
  if (v > 25) return "var(--accent)";
  return "var(--red)";
}

// vitalFillPct clamps a 0–100 vital to a fill percentage (0 when unknown).
export function vitalFillPct(v: number | null): number {
  if (v == null) return 0;
  return Math.max(0, Math.min(100, v));
}
