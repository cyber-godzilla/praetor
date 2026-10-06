import { describe, expect, it } from "vitest";
import { chainStateLabel, chainTimeLabel } from "./chainstatus";
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

describe("chainStateLabel", () => {
  it.each([
    ["pacing", {}, "pacing"],
    ["unbusy", {}, "waiting for unbusy"],
    ["wait", {}, "waiting"],
    ["wait-for", { detail: "ready" }, "waiting for “ready”"],
    ["wait-for", {}, "waiting for “text”"],
    ["repeat", { detail: "climb wall", attempts: 2, maxAttempts: 5 }, "repeating “climb wall” (2/5)"],
    ["repeat", {}, "repeating “command” (1)"],
    ["notify", { detail: "Training" }, "notifying “Training”"],
    ["notify", {}, "notifying “Praetor”"],
    ["send", { detail: "look" }, "sending “look”"],
    ["send", {}, "sending “command”"],
    ["custom", {}, "custom"],
    ["", {}, "starting"],
  ] as const)("renders %s status", (state, fields, want) => {
    expect(chainStateLabel(status(state, fields))).toBe(want);
  });
});
