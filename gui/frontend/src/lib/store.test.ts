import { describe, it, expect, beforeEach } from "vitest";
import { store } from "./store.svelte";
import { Kind } from "./types";
import type { WireEvent } from "./types";
import { webFixtureEvents } from "./web-fixture";

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
    store.connState = "disconnected";
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
    store.apply(conn("connected"));
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

  it("ignores an in-game event that arrives after the disconnect event", () => {
    store.accounts = ["alice"];
    // In game, receive a status update.
    store.apply(conn("connected"));
    store.apply([{ kind: Kind.Status, status: { mode: "hunt" } }]);
    expect(store.mode).toBe("hunt");
    // Disconnect, then a late Status races in AFTER the Conn:disconnected event.
    store.apply(conn("disconnected", "connection closed"));
    expect(store.mode).toBe(""); // resetSession cleared it
    store.apply([{ kind: Kind.Status, status: { mode: "raid" } }]);
    expect(store.mode).toBe(""); // guard dropped the late event
    expect(store.screen).toBe("account"); // still on the bootup screen
  });

  it("applies in-game events normally while connected", () => {
    store.apply(conn("connected"));
    store.apply([{ kind: Kind.Status, status: { mode: "craft" } }]);
    expect(store.mode).toBe("craft");
  });

  it("installs the complete web fixture atomically and idempotently", () => {
    store.debug = true;
    store.rebuildTabs([]);
    store.installSnapshot(webFixtureEvents);
    const firstLineCount = store.tabs.find((tab) => tab.kind === "all")?.lines.length;
    expect(store.connState).toBe("connected");
    expect(store.mode).toBe("fixture");
    expect(store.health).toBe(80);
    expect(store.fatigue).toBe(60);
    expect(store.encumbrance).toBe(40);
    expect(store.satiation).toBe(20);
    expect(store.lightingRaw).toBe(23);
    expect(store.minimap).toContain("data:image/png");
    expect(store.compass).toContain("data:image/png");

    store.installSnapshot(webFixtureEvents);
    expect(store.tabs.find((tab) => tab.kind === "all")?.lines.length).toBe(firstLineCount);
  });

  it("caps scrollback with a batched front-trim (never far above cap, newest kept)", () => {
    store.config = { UI: { Scrollback: 50 } } as any;
    store.rebuildTabs([]);
    store.apply(conn("connected"));

    for (let i = 0; i < 500; i++) {
      store.apply([
        {
          kind: Kind.Text,
          text: { text: `line ${i}`, segments: [{ text: `line ${i}` }], isEcho: false, timestamp: 0 },
        },
      ]);
    }

    const all = store.tabs.find((t) => t.kind === "all")!;
    // Bounded by cap + one trim chunk, and never below cap once past it.
    expect(all.lines.length).toBeLessThanOrEqual(50 + 256);
    expect(all.lines.length).toBeGreaterThanOrEqual(50);
    // The most recent line is always retained.
    const last = all.lines[all.lines.length - 1];
    expect(last.segments[0].text).toBe("line 499");
  });
});

describe("addLocalLine", () => {
  it("appends a dim informational line to the All tab", () => {
    store.rebuildTabs([]);
    const all = store.tabs.find((t) => t.kind === "all")!;
    const before = all.lines.length;
    store.addLocalLine("Note A — hello");
    expect(all.lines.length).toBe(before + 1);
    const last = all.lines[all.lines.length - 1];
    expect(last.segments[0].text).toBe("Note A — hello");
    expect(last.isEcho).toBe(false);
  });
});
