import { afterEach, describe, expect, it, vi } from "vitest";
// The production tsconfig intentionally omits Node types; Vitest still runs in
// Node and provides this built-in for the source-level parity contract.
// @ts-expect-error node:fs types are test-runner-only
import { readFileSync } from "node:fs";
import { settingsOperations, WEB_SUPPORTED_METHODS, WebTransport } from "./transport-web";
import { Kind } from "./types";

describe("web transport operation parity", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("has an explicit web decision for every transport-neutral bridge call", () => {
    const source = readFileSync(new URL("./bridge.ts", import.meta.url), "utf8");
    const methods = new Set<string>();
    for (const match of source.matchAll(/call(?:<[^>]+>)?\(\s*"([^"]+)"/g)) {
      methods.add(match[1]);
    }

    expect(methods.size).toBeGreaterThan(20);
    expect([...methods].filter((method) => !WEB_SUPPORTED_METHODS.has(method))).toEqual([]);
  });

  it("fails closed for an unknown operation", async () => {
    const transport = new WebTransport();
    await expect(transport.invoke("FutureWailsOnlyMethod", undefined)).rejects.toThrow(
      "No web transport operation",
    );
  });

  it("maps every mobile web preference to a revisioned setting operation", () => {
    expect(settingsOperations).toMatchObject({
      SetMobileShowToolbar: "mobile-show-toolbar",
      SetMobileShowTabBar: "mobile-show-tab-bar",
      SetMobileHideNavigationOnInput: "mobile-hide-navigation-on-input",
      SetMobileLowercaseFirstLetter: "mobile-lowercase-first-letter",
      SetMobileOutputFontSize: "mobile-output-font-size",
    });
  });

  it("installs a snapshot before ordered live events and rejects a gap", () => {
    const transport = new WebTransport();
    const received: string[] = [];
    transport.subscribe({
      snapshot: (events) => received.push(`snapshot:${events[0]?.text?.text}`),
      events: (events) => received.push(`events:${events[0]?.text?.text}`),
      system: (update) => {
        if (update.type === "transport") received.push(`transport:${update.transportState}`);
      },
    });
    const envelope = (sequence: number, text: string) => ({
      type: "events",
      protocol: 1,
      serverId: "server-a",
      sequence,
      events: [{ kind: Kind.Text, text: { text, segments: [{ text }] } }],
    });

    (transport as any).handleEnvelope({
      type: "snapshot",
      protocol: 1,
      serverId: "server-a",
      sequence: 4,
      events: [{ kind: Kind.Text, text: { text: "before", segments: [{ text: "before" }] } }],
    });
    (transport as any).handleEnvelope(envelope(5, "after"));

    expect(received).toEqual(["snapshot:before", "transport:connected", "events:after"]);
    expect(() => (transport as any).handleEnvelope(envelope(7, "gap"))).toThrow(
      "sequence gap",
    );
  });

  it("does not roll config backward when an older broadcast follows a mutation response", () => {
    const transport = new WebTransport();
    const revisions: number[] = [];
    transport.subscribe({
      events: () => {},
      system: (update) => {
        if (update.type === "config" && update.revision !== undefined) revisions.push(update.revision);
      },
    });
    const config = {} as any;
    (transport as any).handleEnvelope({
      type: "snapshot", protocol: 1, serverId: "server-a", sequence: 1,
      revision: 1, config,
    });
    (transport as any).acceptConfigMutation({ revision: 3, config });
    (transport as any).handleEnvelope({
      type: "config", protocol: 1, serverId: "server-a", sequence: 2,
      revision: 2, config,
    });

    expect(revisions).toEqual([1, 3]);
  });

  it("returns a successful connection separately from a credential-save warning", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({
      connected: true,
      credentialSaveRequested: true,
      credentialsSaved: false,
      warning: "Connected, but the account was not remembered.",
      accountState: {
        accounts: [],
        credentialStore: {
          backend: "keyring",
          available: false,
          canStore: true,
          message: "The system keyring is unavailable.",
        },
      },
    }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetchMock);

    const transport = new WebTransport();
    const result = await transport.invoke<any>("ConnectNew", undefined, "alice", "password", true);

    expect(result.connected).toBe(true);
    expect(result.credentialsSaved).toBe(false);
    expect(result.warning).toContain("not remembered");
    expect(result.accountState.credentialStore.available).toBe(false);
  });

  it("projects credential-store health with account snapshots", () => {
    const transport = new WebTransport();
    const updates: any[] = [];
    transport.subscribe({
      events: () => {},
      system: (update) => {
        if (update.type === "accounts") updates.push(update);
      },
    });

    (transport as any).handleEnvelope({
      type: "snapshot",
      protocol: 1,
      serverId: "server-a",
      sequence: 1,
      accounts: [],
      credentialStore: {
        backend: "encrypted_file",
        available: false,
        canStore: true,
        message: "Encrypted credential storage is unavailable.",
      },
    });

    expect(updates).toEqual([{
      type: "accounts",
      accounts: [],
      credentialStore: {
        backend: "encrypted_file",
        available: false,
        canStore: true,
        message: "Encrypted credential storage is unavailable.",
      },
    }]);
  });
});
