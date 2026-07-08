import { describe, it, expect, beforeEach } from "vitest";
import { store } from "./store.svelte";
import { Kind } from "./types";
import type { WireEvent } from "./types";

function conn(state: "connected" | "disconnected", reason?: string): WireEvent[] {
  return [{ kind: Kind.Conn, conn: { state, reason } }];
}

describe("store connection routing", () => {
  beforeEach(() => {
    store.config = { UI: { Scrollback: 5000 } } as any;
    store.debug = false;
    store.rebuildTabs([]);
    store.accounts = [];
    store.screen = "loading";
    store.disconnectNotice = "";
  });

  it("routes to game on connect and clears any prior notice", () => {
    store.disconnectNotice = "stale";
    store.apply(conn("connected"));
    expect(store.screen).toBe("game");
    expect(store.disconnectNotice).toBe("");
  });

  it("user logout (empty reason) returns to login with no notice when no accounts", () => {
    store.accounts = [];
    store.apply(conn("disconnected", ""));
    expect(store.screen).toBe("login");
    expect(store.disconnectNotice).toBe("");
  });

  it("user logout returns to the account screen when accounts exist", () => {
    store.accounts = ["alice"];
    store.apply(conn("disconnected", ""));
    expect(store.screen).toBe("account");
    expect(store.disconnectNotice).toBe("");
  });

  it("dropped connection (reason) shows a notice on the bootup screen", () => {
    store.accounts = ["alice"];
    store.apply(conn("disconnected", "connection closed"));
    expect(store.screen).toBe("account");
    expect(store.disconnectNotice).toBe("connection closed");
  });

  it("resetSession clears output, status, mode, and graphics", () => {
    store.apply([
      {
        kind: Kind.Text,
        text: { text: "hello", segments: [{ text: "hello" }], isEcho: false, timestamp: 0 },
      },
    ]);
    store.health = 42;
    store.mode = "hunt";
    store.minimap = "data:image/png;base64,AAAA";
    store.resetSession();
    expect(store.tabs.every((t) => t.lines.length === 0)).toBe(true);
    expect(store.health).toBe(null);
    expect(store.mode).toBe("");
    expect(store.minimap).toBe("");
  });
});
