import { describe, it, expect } from "vitest";
import { fmtDur, currentElapsedMs } from "./metrics";
import type { MetricSession } from "./types";

function session(over: Partial<MetricSession> = {}): MetricSession {
  return { mode: "m", start: 0, end: 0, durationMs: 0, entries: [], ...over };
}

describe("fmtDur", () => {
  it("seconds only", () => expect(fmtDur(5_000)).toBe("5s"));
  it("minutes and seconds", () => expect(fmtDur(125_000)).toBe("2m 5s"));
  it("hours, minutes, seconds", () => expect(fmtDur(3_725_000)).toBe("1h 2m 5s"));
  it("floors sub-second", () => expect(fmtDur(1_900)).toBe("1s"));
});

describe("currentElapsedMs", () => {
  it("counts from start to now for a live session", () =>
    expect(currentElapsedMs(session({ start: 10_000 }), 15_000)).toBe(5_000));

  it("never goes negative if the clock skews behind start", () =>
    expect(currentElapsedMs(session({ start: 20_000 }), 15_000)).toBe(0));

  it("falls back to server durationMs when start is missing", () =>
    expect(currentElapsedMs(session({ start: 0, durationMs: 42_000 }), 99_000)).toBe(42_000));
});
