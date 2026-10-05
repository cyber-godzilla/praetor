import { afterEach, describe, expect, it, vi } from "vitest";
// The production tsconfig intentionally omits Node types; Vitest still runs in
// Node and provides this built-in for the source-level parity contract.
import { readFileSync } from "node:fs";
import { settingsOperations, WEB_SUPPORTED_METHODS, WebTransport } from "./transport-web";
import { WebAuthRequiredError } from "./transport";
import { Kind } from "./types";

describe("web transport operation parity", () => {
  afterEach(() => {
    FakeBroadcastChannel.reset();
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

  it("maps shared and mobile preferences to revisioned setting operations", () => {
    expect(settingsOperations).toMatchObject({
      SetInputSpellcheck: "input-spellcheck",
      SetUpdateCheck: "update-check",
      SetMobileShowToolbar: "mobile-show-toolbar",
      SetMobileShowTabBar: "mobile-show-tab-bar",
      SetMobileHideNavigationOnInput: "mobile-hide-navigation-on-input",
      SetMobileLowercaseFirstLetter: "mobile-lowercase-first-letter",
      SetMobileOutputFontSize: "mobile-output-font-size",
      SetRetainAppLogs: "retain-app-logs",
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

  it("refreshes a rejected CSRF token and retries the mutation exactly once", async () => {
    const requests: Array<{ url: string; csrf: string; body: string }> = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const csrf = new Headers(init?.headers).get("X-Praetor-CSRF") ?? "";
      requests.push({ url, csrf, body: String(init?.body ?? "") });
      if (url === "/api/v1/bootstrap") {
        return jsonResponse(bootstrap("csrf-new"));
      }
      if (url === "/api/v1/commands" && csrf === "csrf-old") {
        return apiError(403, "csrf_rejected", "Request verification failed.");
      }
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old");
    await transport.invoke("Send", undefined, "look");

    expect(requests.map(({ url }) => url)).toEqual([
      "/api/v1/commands",
      "/api/v1/bootstrap",
      "/api/v1/commands",
    ]);
    expect(requests.map(({ csrf }) => csrf)).toEqual([
      "csrf-old",
      "",
      "csrf-new",
    ]);
    expect(requests[0].body).toBe(requests[2].body);
  });

  it("never retries a non-CSRF forbidden response", async () => {
    const fetchMock = vi.fn(async () =>
      apiError(403, "origin_rejected", "Request origin rejected."));
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old");
    await expect(transport.invoke("Send", undefined, "look")).rejects.toThrow(
      "Request origin rejected.",
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("bounds repeated CSRF rejection to one bootstrap and one retry", async () => {
    const urls: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      urls.push(url);
      if (url === "/api/v1/bootstrap") {
        return jsonResponse(bootstrap("csrf-new"));
      }
      return apiError(403, "csrf_rejected", "Request verification failed.");
    });
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old");
    await expect(transport.invoke("Send", undefined, "look")).rejects.toThrow(
      "Request verification failed.",
    );
    expect(urls).toEqual([
      "/api/v1/commands",
      "/api/v1/bootstrap",
      "/api/v1/commands",
    ]);
  });

  it("does not replay a mutation when recovery discovers a new server process", async () => {
    const urls: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      urls.push(url);
      if (url === "/api/v1/bootstrap") {
        return jsonResponse(bootstrap("csrf-new", "server-b"));
      }
      return apiError(403, "csrf_rejected", "Request verification failed.");
    });
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old", "server-a");
    await expect(transport.invoke("Send", undefined, "look")).rejects.toThrow(
      "Praetor restarted and is resynchronizing.",
    );
    expect(urls).toEqual([
      "/api/v1/commands",
      "/api/v1/bootstrap",
    ]);
  });

  it("coalesces concurrent CSRF recovery without duplicating mutations", async () => {
    let releaseBootstrap: ((response: Response) => void) | undefined;
    const bootstrapResponse = new Promise<Response>((resolve) => {
      releaseBootstrap = resolve;
    });
    let bootstrapRequests = 0;
    const commandCSRF: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/v1/bootstrap") {
        bootstrapRequests++;
        return bootstrapResponse;
      }
      const csrf = new Headers(init?.headers).get("X-Praetor-CSRF") ?? "";
      commandCSRF.push(csrf);
      if (csrf === "csrf-old") {
        return apiError(403, "csrf_rejected", "Request verification failed.");
      }
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old");
    const first = transport.invoke("Send", undefined, "look");
    const second = transport.invoke("Send", undefined, "inventory");
    await vi.waitFor(() => {
      expect(commandCSRF).toEqual(["csrf-old", "csrf-old"]);
      expect(bootstrapRequests).toBe(1);
    });
    releaseBootstrap?.(jsonResponse(bootstrap("csrf-new")));

    await Promise.all([first, second]);
    expect(bootstrapRequests).toBe(1);
    expect(commandCSRF).toEqual([
      "csrf-old",
      "csrf-old",
      "csrf-new",
      "csrf-new",
    ]);
  });

  it("enters web authentication when CSRF refresh finds an expired session", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/api/v1/bootstrap") {
        return apiError(401, "authentication_required", "Authentication required.");
      }
      return apiError(403, "csrf_rejected", "Request verification failed.");
    });
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old");
    const updates: string[] = [];
    transport.subscribe({
      events: () => {},
      system: (update) => updates.push(update.type),
    });

    await expect(transport.invoke("Send", undefined, "look")).rejects.toBeInstanceOf(
      WebAuthRequiredError,
    );
    expect(updates).toEqual(["auth-expired"]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("fails closed when CSRF recovery receives an invalid bootstrap", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/api/v1/bootstrap") {
        return jsonResponse({ ...bootstrap("", ""), csrf: "", serverId: "" });
      }
      return apiError(403, "csrf_rejected", "Request verification failed.");
    });
    vi.stubGlobal("fetch", fetchMock);

    const transport = initializedTransport("csrf-old");
    const updates: string[] = [];
    transport.subscribe({
      events: () => {},
      system: (update) => updates.push(update.type),
    });

    await expect(transport.invoke("Send", undefined, "look")).rejects.toBeInstanceOf(
      WebAuthRequiredError,
    );
    expect(updates).toEqual(["auth-expired"]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("preserves the typed login failure instead of treating it as session expiry", async () => {
    const fetchMock = vi.fn(async () =>
      apiError(401, "login_failed", "Authentication failed."));
    vi.stubGlobal("fetch", fetchMock);

    const transport = new WebTransport();
    const updates: string[] = [];
    transport.subscribe({
      events: () => {},
      system: (update) => updates.push(update.type),
    });

    await expect(transport.webLogin("wrong")).rejects.toThrow(
      "Authentication failed.",
    );
    expect(updates).toEqual([]);
  });

  it("refreshes another same-profile tab after login without sharing a token", async () => {
    vi.stubGlobal("window", {});
    vi.stubGlobal("BroadcastChannel", FakeBroadcastChannel);
    const requestCSRF: string[] = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/v1/auth/login") return jsonResponse({ ok: true });
      if (url === "/api/v1/bootstrap") {
        return jsonResponse(bootstrap("csrf-replacement"));
      }
      requestCSRF.push(
        new Headers(init?.headers).get("X-Praetor-CSRF") ?? "",
      );
      return jsonResponse({ ok: true });
    });
    vi.stubGlobal("fetch", fetchMock);

    const existingTab = initializedTransport("csrf-original");
    const systemUpdates: string[] = [];
    existingTab.subscribe({
      events: () => {},
      system: (update) => systemUpdates.push(update.type),
    });
    const loginTab = new WebTransport();

    await loginTab.webLogin("correct");
    await vi.waitFor(() => {
      expect(systemUpdates).toContain("auth-restored");
    });
    await existingTab.invoke("Send", undefined, "look");

    expect(requestCSRF).toEqual(["csrf-replacement"]);
    expect(FakeBroadcastChannel.messages).toHaveLength(1);
    expect(FakeBroadcastChannel.messages[0]).toMatchObject({
      type: "praetor-session-changed",
      action: "login",
    });
    expect(FakeBroadcastChannel.messages[0]).not.toHaveProperty("csrf");
    expect(FakeBroadcastChannel.messages[0]).not.toHaveProperty("password");
  });

  it("signs out other tabs in the same browser profile", async () => {
    vi.stubGlobal("window", {});
    vi.stubGlobal("BroadcastChannel", FakeBroadcastChannel);
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse({ ok: true })));

    const otherTab = initializedTransport("csrf-current");
    const updates: string[] = [];
    otherTab.subscribe({
      events: () => {},
      system: (update) => updates.push(update.type),
    });
    const signingOutTab = initializedTransport("csrf-current");

    await signingOutTab.webLogout();
    await vi.waitFor(() => {
      expect(updates).toEqual(["auth-expired"]);
    });
  });
});

function initializedTransport(csrf: string, serverId = "server-a"): WebTransport {
  const transport = new WebTransport();
  (transport as any).installBootstrap(bootstrap(csrf, serverId));
  return transport;
}

function bootstrap(csrf: string, serverId = "server-a") {
  return {
    protocol: 1,
    csrf,
    serverId,
    configRevision: 1,
    version: "test",
    debug: false,
    accounts: [],
    modeNames: [],
    hasModes: false,
    config: {},
  };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function apiError(status: number, code: string, message: string): Response {
  return jsonResponse({ error: { code, message } }, status);
}

class FakeBroadcastChannel {
  static channels = new Set<FakeBroadcastChannel>();
  static messages: unknown[] = [];

  onmessage: ((event: MessageEvent<unknown>) => void) | null = null;

  constructor(readonly name: string) {
    FakeBroadcastChannel.channels.add(this);
  }

  postMessage(message: unknown) {
    FakeBroadcastChannel.messages.push(message);
    for (const channel of FakeBroadcastChannel.channels) {
      if (channel === this || channel.name !== this.name) continue;
      queueMicrotask(() => channel.onmessage?.({ data: message } as MessageEvent));
    }
  }

  close() {
    FakeBroadcastChannel.channels.delete(this);
  }

  static reset() {
    FakeBroadcastChannel.channels.clear();
    FakeBroadcastChannel.messages = [];
  }
}
