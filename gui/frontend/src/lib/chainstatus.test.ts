import { describe, expect, it } from "vitest";
import { chainTimeLabel } from "./chainstatus";
import type { InputChainStatus } from "./types";

function status(state: string, fields: Partial<InputChainStatus>): InputChainStatus {
  return { active: true, chains: 1, step: 1, total: 2, state, ...fields };
}

describe("chainTimeLabel", () => {
  it("keeps the initial pacing duration fixed", () => {
    expect(chainTimeLabel(status("pacing", { durationMs: 900, remainingMs: 899 }))).toBe("0.9s");
    expect(chainTimeLabel(status("pacing", { durationMs: 900, remainingMs: 101 }))).toBe("0.9s");
    expect(chainTimeLabel(status("pacing", { durationMs: 1250, remainingMs: 4 }))).toBe("1.25s");
  });

  it("counts waits down in whole seconds", () => {
    expect(chainTimeLabel(status("wait", { remainingMs: 29_500 }))).toBe("30s");
    expect(chainTimeLabel(status("wait", { remainingMs: 9_000 }))).toBe("9s");
    expect(chainTimeLabel(status("wait-for", { remainingMs: 8_001 }))).toBe("9s");
    expect(chainTimeLabel(status("wait-for", { remainingMs: 500 }))).toBe("1s");
  });

  it("does not invent a time for untimed states", () => {
    expect(chainTimeLabel(status("unbusy", {}))).toBe("");
    expect(chainTimeLabel(status("repeat", {}))).toBe("");
  });
});
