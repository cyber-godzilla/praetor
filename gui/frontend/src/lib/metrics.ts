// Metrics-panel duration helpers shared by MetricsPanel and its tests.

import type { MetricSession } from "./types";

// fmtDur renders a millisecond span as a compact "1h 2m 5s" / "2m 5s" / "5s"
// string, flooring sub-second remainders.
export function fmtDur(ms: number): string {
  const s = Math.floor(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h > 0) return `${h}h ${m}m ${sec}s`;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
}

// currentElapsedMs returns the live elapsed time of an active session, counting
// from its start up to `now` (both unix millis). The clock is presentational, so
// it never returns a negative span if `now` skews behind start. When start is
// missing (0), it falls back to the server-computed durationMs snapshot.
export function currentElapsedMs(session: MetricSession, now: number): number {
  if (!session.start) return session.durationMs;
  return Math.max(0, now - session.start);
}
