import type { InputChainStatus } from "./types";

export function chainStateLabel(status: InputChainStatus): string {
  switch (status.state) {
    case "pacing": return "pacing";
    case "unbusy": return "waiting for unbusy";
    case "wait": return "waiting";
    case "wait-for": return `waiting for “${status.detail ?? "text"}”`;
    case "repeat": {
      const limit = status.maxAttempts ? `/${status.maxAttempts}` : "";
      return `repeating “${status.detail ?? "command"}” (${status.attempts ?? 1}${limit})`;
    }
    case "notify": return `notifying “${status.detail ?? "Praetor"}”`;
    case "send": return `sending “${status.detail ?? "command"}”`;
    default: return status.state || "starting";
  }
}

// Pacing is a configured delay, so show the original duration without making
// it look like a countdown. True waits show only whole seconds and round up so
// a fresh 30-second wait starts at 30 rather than flashing 29 immediately.
export function chainTimeLabel(status: InputChainStatus): string {
  if (status.state === "pacing") {
    if (!status.durationMs) return "";
    const seconds = status.durationMs / 1000;
    return `${Number(seconds.toFixed(3))}s`;
  }
  if (status.remainingMs === undefined || status.remainingMs <= 0) return "";
  return `${Math.ceil(status.remainingMs / 1000)}s`;
}
