import type {
  AppConfig,
  AccountState,
  ConnectResult,
  CredentialStoreStatus,
  DesktopNotificationsConfig,
  InitState,
  KudosConfig,
  WireEvent,
} from "./types";
import type {
  PraetorTransport,
  SystemUpdate,
  TransportHandlers,
  WebBootstrap,
} from "./transport";
import { WebAuthRequiredError } from "./transport";

interface WebEnvelope {
  type: "snapshot" | "events" | "config" | "modes" | "accounts" | "operation";
  protocol: number;
  serverId: string;
  sequence?: number;
  fromSequence?: number;
  toSequence?: number;
  events?: WireEvent[];
  config?: AppConfig;
  revision?: number;
  modeNames?: string[];
  accounts?: string[];
  credentialStore?: CredentialStoreStatus;
  result?: { operation: string; ok: boolean; message?: string };
}

interface ErrorResponse {
  error?: { code?: string; message?: string; requestId?: string };
}

interface SessionSignal {
  type: "praetor-session-changed";
  action: "login" | "logout";
  source: string;
  generation: string;
}

interface BootstrapRefresh {
  init: WebBootstrap;
  serverChanged: boolean;
}

const sessionChannelName = "praetor-web-session-v1";

class WebAPIError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "WebAPIError";
    this.status = status;
    this.code = code;
  }
}

export class WebTransport implements PraetorTransport {
  readonly kind = "web" as const;

  private csrf = "";
  private revision = 0;
  private serverId = "";
  private sequence = 0;
  private socket: WebSocket | null = null;
  private handlers = new Set<TransportHandlers>();
  private started = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private socketReady = false;
  private csrfRefresh: Promise<BootstrapRefresh> | null = null;
  private authExpired = false;
  private readonly sessionSource = randomSessionMarker();
  private sessionChannel: BroadcastChannel | null = null;

  constructor() {
    if (
      typeof window !== "undefined" &&
      typeof BroadcastChannel !== "undefined"
    ) {
      try {
        this.sessionChannel = new BroadcastChannel(sessionChannelName);
        this.sessionChannel.onmessage = (event: MessageEvent<unknown>) => {
          const signal = parseSessionSignal(event.data);
          if (!signal || signal.source === this.sessionSource) return;
          void this.handleSessionSignal(signal);
        };
      } catch {
        // Some embedded/private browser contexts expose BroadcastChannel but
        // deny its construction. Typed CSRF recovery remains the fallback.
        this.sessionChannel = null;
      }
    }
  }

  async invoke<T>(method: string, fallback: T, ...args: any[]): Promise<T> {
    switch (method) {
      case "GetInitState": {
        const init = await this.request<WebBootstrap>("GET", "/api/v1/bootstrap");
        this.installBootstrap(init);
        return init as T;
      }
      case "GetConfig": {
        const init = await this.request<WebBootstrap>("GET", "/api/v1/bootstrap");
        this.installBootstrap(init);
        return init.config as T;
      }
      case "ListAccounts": {
        return (await this.request<AccountState>("GET", "/api/v1/accounts")) as T;
      }
      case "ConnectNew":
        return (await this.request<ConnectResult>("POST", "/api/v1/game/connect", {
          username: args[0],
          password: args[1],
          store: args[2],
        })) as T;
      case "ConnectStored":
        await this.request("POST", "/api/v1/game/connect-stored", { username: args[0] });
        return undefined as T;
      case "SaveAccount":
        await this.request("PUT", `/api/v1/accounts/${encodeURIComponent(args[0])}`, { password: args[1] });
        return undefined as T;
      case "RemoveAccount":
        await this.request("DELETE", `/api/v1/accounts/${encodeURIComponent(args[0])}`);
        return undefined as T;
      case "Disconnect":
        await this.request("POST", "/api/v1/game/disconnect", {});
        return undefined as T;
      case "Send":
        await this.request("POST", "/api/v1/commands", { input: args[0] });
        return undefined as T;
      case "ModeNames": {
        const data = await this.request<{ modeNames: string[] }>("GET", "/api/v1/modes");
        return (data.modeNames ?? []) as T;
      }
      case "CurrentMode": {
        const data = await this.request<{ currentMode: string }>("GET", "/api/v1/modes");
        return (data.currentMode ?? "") as T;
      }
      case "SetMode":
        await this.request("PUT", "/api/v1/mode", { name: args[0], args: args[1] });
        return undefined as T;
      case "ReloadScripts":
        await this.request("POST", "/api/v1/scripts/reload", {});
        return undefined as T;
      case "PickScriptDir":
        // Browser clients cannot open a native picker on the server host. The
        // web scripts editor accepts server-side paths as text instead.
        return fallback;
      case "PickSendFile":
      case "PickPlayFile":
        // /send and /play read local files through a native picker and have no
        // praetor-web endpoints yet. Both call sites catch and toast, so the
        // browser user gets an explanation instead of a silent cancel.
        throw new Error("/send and /play scripts are not available in the browser client yet.");
      case "StartFileSend":
      case "StartPlay":
        // Unreachable in the browser: starting either flow requires a
        // successful pick above. The explicit decision keeps the parity
        // contract honest.
        return fallback;
      case "AbortSend":
      case "PausePlay":
      case "ResumePlay":
      case "StopPlay":
      case "NextPlayStep":
      case "PlayActive":
      case "PlayStatus":
        // No file send or performance can exist in a browser session. These
        // report the idle state (false / inactive status) so Alt+X, the
        // pre-submit play gate, and the slash controls behave exactly like an
        // idle desktop instead of erroring.
        return fallback;
      case "RefreshGraphics":
        await this.request("POST", "/api/v1/graphics/refresh", {});
        return undefined as T;
      case "ClipboardGet":
        if (!navigator.clipboard?.readText) throw new Error("Browser clipboard read is unavailable; use the browser's Paste command.");
        return (await navigator.clipboard.readText()) as T;
      case "ClipboardSet":
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(args[0]);
        } else {
          this.copyFallback(args[0]);
        }
        return undefined as T;
      case "GetKudos":
        return (await this.request<KudosConfig>("GET", "/api/v1/kudos")) as T;
      case "SetKudos": {
        const result = await this.request<ConfigMutationResponse>("PUT", "/api/v1/kudos", {
          expectedRevision: this.revision,
          value: args[0],
        });
        this.acceptConfigMutation(result);
        return undefined as T;
      }
      case "AddKudosFavorite": {
        const result = await this.request<ConfigMutationResponse & { added: boolean }>(
          "POST",
          "/api/v1/kudos/favorites",
          { name: args[0] },
        );
        this.acceptConfigMutation(result);
        return result.added as T;
      }
      case "AddKudosQueue": {
        const result = await this.request<ConfigMutationResponse>(
          "POST",
          "/api/v1/kudos/queue",
          { name: args[0], message: args[1] },
        );
        this.acceptConfigMutation(result);
        return undefined as T;
      }
      case "GetPersistentData":
        return (await this.request("GET", "/api/v1/persistent")) as T;
      case "ExportPersistentData":
        return (await this.downloadPersistent(args[0])) as T;
      case "ClearPersistentData":
        await this.request("DELETE", "/api/v1/persistent", { keys: args[0] });
        return undefined as T;
      case "ListNotes":
        return (await this.request("GET", "/api/v1/notes")) as T;
      case "GetNote":
        return (await this.request("GET", `/api/v1/notes/${encodeURIComponent(args[0])}`)) as T;
      case "SaveNote":
        await this.request("PUT", "/api/v1/notes", {
          originalTitle: args[0],
          title: args[1],
          body: args[2],
        });
        return undefined as T;
      case "DeleteNote":
        await this.request("DELETE", `/api/v1/notes/${encodeURIComponent(args[0])}`);
        return undefined as T;
      case "GetWikiSections":
        return (await this.request("GET", "/api/v1/wiki")) as T;
      case "GetMapSections":
        return (await this.request("GET", "/api/v1/maps")) as T;
      case "OpenURL":
        this.openURL(args[0]);
        return undefined as T;
      case "OpenWikiSlug":
        this.openURL(`http://eternal-city.wikidot.com/${encodeURIComponent(args[0])}`);
        return undefined as T;
      case "CalcRankBonus":
        return (await this.request("POST", "/api/v1/calc/rank-bonus", {
          mode: args[0], basics: args[1], subskill: args[2],
        })) as T;
      case "CalcTrainCost":
        return (await this.request("POST", "/api/v1/calc/train-cost", {
          current: args[0], desired: args[1], slot: args[2], difficulty: args[3],
          selfTrained: args[4], selfTaught: args[5], healing: args[6],
        })) as T;
      case "CheckForUpdate":
        // Startup update checks are intentionally owned by the native shell;
        // do not repeat them once per connected browser.
        return fallback;
      default:
        if (settingsOperations[method]) {
          await this.updateSetting(settingsOperations[method], settingPayload(method, args));
          return undefined as T;
        }
        throw new Error(`No web transport operation for ${method}`);
    }
  }

  subscribe(handlers: TransportHandlers): () => void {
    this.handlers.add(handlers);
    return () => this.handlers.delete(handlers);
  }

  async start(): Promise<void> {
    this.started = true;
    this.socketReady = false;
    this.emitSystem({ type: "transport", transportState: "connecting" });
    this.openSocket();
  }

  async webLogin(password: string): Promise<void> {
    await this.request("POST", "/api/v1/auth/login", { password }, false);
    this.started = false;
    this.authExpired = false;
    this.reconnectAttempt = 0;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    this.broadcastSessionChange("login");
  }

  async webLogout(): Promise<void> {
    try {
      await this.request("POST", "/api/v1/auth/logout", {});
    } catch (error) {
      // Local sign-out must remain available during a network failure. The
      // opaque HttpOnly cookie cannot be cleared client-side, but it expires
      // with the in-memory server process and a later login replaces it.
      console.warn("Praetor logout request did not complete:", error);
    } finally {
      this.broadcastSessionChange("logout");
      this.expireAuthentication("signed out");
    }
  }

  async quit(): Promise<void> {
    await this.webLogout();
  }

  showLocalNotification(title: string, message: string): void {
    if ("Notification" in window && Notification.permission === "granted") {
      new Notification(title, { body: message });
    }
  }

  async requestNotificationPermission(): Promise<NotificationPermission | "unsupported"> {
    if (!("Notification" in window) || !window.isSecureContext) return "unsupported";
    return Notification.requestPermission();
  }

  private installBootstrap(init: WebBootstrap) {
    if (init.protocol !== 1) {
      throw new Error(`Unsupported Praetor web protocol ${init.protocol}`);
    }
    if (
      typeof init.csrf !== "string" ||
      init.csrf === "" ||
      typeof init.serverId !== "string" ||
      init.serverId === "" ||
      typeof init.configRevision !== "number" ||
      !Number.isSafeInteger(init.configRevision) ||
      init.configRevision < 0
    ) {
      throw new Error("Invalid Praetor web bootstrap");
    }
    this.csrf = init.csrf;
    this.revision = init.configRevision;
    this.serverId = init.serverId;
    this.authExpired = false;
  }

  private openSocket() {
    if (!this.started || this.socket || !this.csrf) return;
    const scheme = location.protocol === "https:" ? "wss:" : "ws:";
    const socket = new WebSocket(`${scheme}//${location.host}/api/v1/events`);
    this.socket = socket;
    socket.onopen = () => {
      this.reconnectAttempt = 0;
    };
    socket.onmessage = (event) => {
      try {
        this.handleEnvelope(JSON.parse(event.data) as WebEnvelope);
      } catch (error) {
        console.error("Invalid Praetor event envelope:", error);
        socket.close(1002, "invalid event envelope");
      }
    };
    socket.onclose = () => {
      // A deliberate session refresh can replace the socket before the old
      // close event arrives. That retired socket must not mark its replacement
      // disconnected or schedule an extra reconnect.
      if (this.socket !== socket) return;
      this.socket = null;
      this.socketReady = false;
      if (this.started) {
        this.emitSystem({ type: "transport", transportState: "reconnecting" });
        this.scheduleReconnect();
      }
    };
    socket.onerror = () => socket.close();
  }

  private handleEnvelope(message: WebEnvelope) {
    if (message.protocol !== 1) throw new Error(`Unsupported protocol ${message.protocol}`);
    if (message.type === "snapshot") {
      this.serverId = message.serverId;
      this.sequence = message.sequence ?? 0;
      if (message.config) {
        this.revision = message.revision ?? this.revision;
        this.emitSystem({ type: "config", config: message.config, revision: this.revision });
      }
      if (message.modeNames) this.emitSystem({ type: "modes", modeNames: message.modeNames });
      if (message.accounts || message.credentialStore) {
        this.emitSystem({
          type: "accounts",
          accounts: message.accounts ?? [],
          credentialStore: message.credentialStore,
        });
      }
      for (const handler of this.handlers) handler.snapshot?.(message.events ?? []);
      this.socketReady = true;
      this.emitSystem({ type: "transport", transportState: "connected" });
      return;
    }
    if (message.serverId !== this.serverId || (message.sequence ?? 0) !== this.sequence + 1) {
      throw new Error("Praetor event sequence gap; resynchronizing");
    }
    this.sequence = message.sequence ?? this.sequence;
    if (message.type === "events") {
      for (const handler of this.handlers) handler.events(message.events ?? []);
    } else if (message.type === "config" && message.config) {
      const revision = message.revision ?? this.revision;
      // A mutation response can reach its initiating browser before an older
      // queued WebSocket broadcast. Consume the sequence but never roll the
      // browser's authoritative config revision backward.
      if (revision >= this.revision) {
        this.revision = revision;
        this.emitSystem({ type: "config", config: message.config, revision });
      }
    } else if (message.type === "modes") {
      this.emitSystem({ type: "modes", modeNames: message.modeNames ?? [], result: message.result });
    } else if (message.type === "accounts") {
      this.emitSystem({
        type: "accounts",
        accounts: message.accounts ?? [],
        credentialStore: message.credentialStore,
      });
    } else if (message.type === "operation") {
      this.emitSystem({ type: "operation", result: message.result });
    }
  }

  private emitSystem(update: SystemUpdate) {
    for (const handler of this.handlers) handler.system?.(update);
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return;
    const base = Math.min(30000, 500 * 2 ** this.reconnectAttempt++);
    const delay = base + Math.floor(Math.random() * Math.max(100, base / 4));
    this.reconnectTimer = setTimeout(async () => {
      this.reconnectTimer = null;
      try {
        const init = await this.request<WebBootstrap>("GET", "/api/v1/bootstrap");
        this.installBootstrap(init);
        this.openSocket();
      } catch (error) {
        if (error instanceof WebAuthRequiredError) {
          this.expireAuthentication("authentication expired");
        } else if (this.started) {
          this.scheduleReconnect();
        }
      }
    }, delay);
  }

  private async request<T = unknown>(
    method: string,
    url: string,
    body?: unknown,
    authenticated = true,
  ): Promise<T> {
    const response = await this.fetchResponse(method, url, body, authenticated);
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  }

  private async fetchResponse(
    method: string,
    url: string,
    body?: unknown,
    authenticated = true,
    allowCSRFRecovery = true,
    bypassSocketGate = false,
  ): Promise<Response> {
    if (
      authenticated &&
      method !== "GET" &&
      method !== "HEAD" &&
      url !== "/api/v1/auth/logout" &&
      this.started &&
      !this.socketReady &&
      !bypassSocketGate
    ) {
      throw new Error("Praetor is reconnecting; wait for current state before making changes.");
    }
    const headers: Record<string, string> = { Accept: "application/json" };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    const mutating = method !== "GET" && method !== "HEAD";
    const sentCSRF = authenticated && mutating ? this.csrf : "";
    if (sentCSRF) headers["X-Praetor-CSRF"] = sentCSRF;
    const response = await fetch(url, {
      method,
      headers,
      credentials: "same-origin",
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (response.status === 401 && authenticated) {
      this.expireAuthentication("authentication expired");
      throw new WebAuthRequiredError();
    }
    if (!response.ok) {
      const error = await responseError(response);
      if (
        authenticated &&
        mutating &&
        allowCSRFRecovery &&
        error.status === 403 &&
        error.code === "csrf_rejected"
      ) {
        let serverChanged = false;
        // Another concurrent request may already have installed the new token
        // by the time this rejection arrives. In that case retry directly
        // instead of performing a redundant bootstrap.
        if (!sentCSRF || sentCSRF === this.csrf) {
          ({ serverChanged } = await this.refreshBootstrap());
        }
        if (serverChanged) {
          this.restartSocket("server restarted");
          throw new Error(
            "Praetor restarted and is resynchronizing. Review current state, then try again.",
          );
        }
        // A same-profile login signal may be replacing the event socket at
        // this point. The one replay is still safe: the rejected request never
        // reached its handler, the server process is unchanged, and the fresh
        // bootstrap authenticated the cookie now used by fetch.
        return this.fetchResponse(
          method,
          url,
          body,
          authenticated,
          false,
          true,
        );
      }
      throw error;
    }
    return response;
  }

  private async refreshBootstrap(): Promise<BootstrapRefresh> {
    if (this.csrfRefresh) return this.csrfRefresh;
    this.csrfRefresh = (async () => {
      const previousServerID = this.serverId;
      const response = await this.fetchResponse(
        "GET",
        "/api/v1/bootstrap",
        undefined,
        true,
        false,
      );
      let init: WebBootstrap;
      try {
        init = (await response.json()) as WebBootstrap;
        this.installBootstrap(init);
      } catch {
        this.expireAuthentication("invalid authentication bootstrap");
        throw new WebAuthRequiredError();
      }
      return {
        init,
        serverChanged:
          previousServerID !== "" && previousServerID !== init.serverId,
      };
    })();
    try {
      return await this.csrfRefresh;
    } finally {
      this.csrfRefresh = null;
    }
  }

  private async handleSessionSignal(signal: SessionSignal) {
    if (signal.action === "logout") {
      this.expireAuthentication("signed out in another tab");
      return;
    }
    try {
      const { serverChanged } = await this.refreshBootstrap();
      if (this.started) {
        this.restartSocket(
          serverChanged ? "server session changed" : "browser session changed",
        );
      } else {
        this.emitSystem({ type: "auth-restored" });
      }
    } catch (error) {
      if (!(error instanceof WebAuthRequiredError)) {
        console.warn("Praetor session refresh did not complete:", error);
      }
    }
  }

  private restartSocket(reason: string) {
    const socket = this.socket;
    this.socket = null;
    this.socketReady = false;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    socket?.close(1000, reason);
    if (this.started) {
      this.emitSystem({ type: "transport", transportState: "reconnecting" });
      this.openSocket();
    }
  }

  private expireAuthentication(reason: string) {
    this.started = false;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    this.socket?.close(1000, reason);
    this.socket = null;
    this.csrf = "";
    this.serverId = "";
    this.sequence = 0;
    this.socketReady = false;
    this.csrfRefresh = null;
    if (!this.authExpired) {
      this.authExpired = true;
      this.emitSystem({ type: "auth-expired" });
    }
  }

  private broadcastSessionChange(action: SessionSignal["action"]) {
    try {
      this.sessionChannel?.postMessage({
        type: "praetor-session-changed",
        action,
        source: this.sessionSource,
        generation: randomSessionMarker(),
      } satisfies SessionSignal);
    } catch {
      // Other tabs still recover reactively if a channel closes unexpectedly.
    }
  }

  private async updateSetting(operation: string, value: unknown) {
    const response = await this.request<ConfigMutationResponse>(
      "PUT",
      `/api/v1/settings/${operation}`,
      { expectedRevision: this.revision, value },
    );
    this.acceptConfigMutation(response);
  }

  private acceptConfigMutation(response: ConfigMutationResponse) {
    this.revision = response.revision;
    this.emitSystem({ type: "config", config: response.config, revision: response.revision });
  }

  private async downloadPersistent(keys: string[]): Promise<string> {
    const response = await this.fetchResponse(
      "POST",
      "/api/v1/persistent/export",
      { keys },
    );
    const blob = await response.blob();
    const disposition = response.headers.get("Content-Disposition") ?? "";
    const filename = disposition.match(/filename="?([^";]+)"?/)?.[1] ?? "persistent.json";
    const objectURL = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = objectURL;
    link.download = filename;
    link.click();
    setTimeout(() => URL.revokeObjectURL(objectURL), 0);
    return filename;
  }

  private openURL(url: string) {
    const parsed = new URL(url, window.location.href);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") throw new Error("Unsupported URL scheme");
    window.open(parsed.toString(), "_blank", "noopener,noreferrer");
  }

  private copyFallback(value: string) {
    const field = document.createElement("textarea");
    field.value = value;
    field.setAttribute("readonly", "");
    field.style.position = "fixed";
    field.style.opacity = "0";
    document.body.appendChild(field);
    field.select();
    const copied = document.execCommand("copy");
    field.remove();
    if (!copied) throw new Error("Browser clipboard write is unavailable; copy the selected text manually.");
  }
}

async function responseError(response: Response): Promise<WebAPIError> {
  let detail: ErrorResponse = {};
  try {
    detail = await response.json();
  } catch {
    // Use the status fallback when the response is not a typed API error.
  }
  return new WebAPIError(
    response.status,
    detail.error?.code ?? "request_failed",
    detail.error?.message ?? `Request failed (${response.status})`,
  );
}

function parseSessionSignal(value: unknown): SessionSignal | null {
  if (!value || typeof value !== "object") return null;
  const signal = value as Partial<SessionSignal>;
  if (
    signal.type !== "praetor-session-changed" ||
    (signal.action !== "login" && signal.action !== "logout") ||
    typeof signal.source !== "string" ||
    signal.source === "" ||
    typeof signal.generation !== "string" ||
    signal.generation === ""
  ) {
    return null;
  }
  return signal as SessionSignal;
}

function randomSessionMarker(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  const values = new Uint32Array(4);
  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    crypto.getRandomValues(values);
    return [...values].map((value) => value.toString(16).padStart(8, "0")).join("");
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

interface ConfigMutationResponse {
  config: AppConfig;
  revision: number;
}

export const settingsOperations: Record<string, string> = {
  SetEchoTyped: "echo-typed",
  SetEchoScript: "echo-script",
  SetColorWords: "color-words",
  SetHideIPs: "hide-ips",
  SetInputSpellcheck: "input-spellcheck",
  SetUpdateCheck: "update-check",
  SetMobileShowToolbar: "mobile-show-toolbar",
  SetMobileShowTabBar: "mobile-show-tab-bar",
  SetMobileHideNavigationOnInput: "mobile-hide-navigation-on-input",
  SetMobileLowercaseFirstLetter: "mobile-lowercase-first-letter",
  SetMobileOutputFontSize: "mobile-output-font-size",
  SetRetainAppLogs: "retain-app-logs",
  SetSessionLogging: "session-logging",
  SetLogPath: "log-path",
  SetDisplayMode: "display-mode",
  SetNumpadNavigation: "numpad-navigation",
  SetMinimapScale: "minimap-scale",
  SetCompassScale: "compass-scale",
  SetOutputFontSize: "output-font-size",
  SetCRTEffects: "crt-effects",
  SetHighlights: "highlights",
  SetCustomTabs: "custom-tabs",
  SetActionSets: "action-sets",
  SetQuickCycleModes: "quick-cycle-modes",
  SetHighPriority: "high-priority",
  SetIgnoreOOC: "ignore-ooc",
  SetIgnoreThink: "ignore-think",
  SetNotifications: "notifications",
  SetScriptDirs: "script-directories",
};

export const WEB_SUPPORTED_METHODS = new Set([
  "GetInitState",
  "GetConfig",
  "ListAccounts",
  "ConnectNew",
  "ConnectStored",
  "SaveAccount",
  "RemoveAccount",
  "Disconnect",
  "Send",
  "ModeNames",
  "CurrentMode",
  "SetMode",
  "ReloadScripts",
  "PickScriptDir",
  "PickSendFile",
  "StartFileSend",
  "AbortSend",
  "PickPlayFile",
  "StartPlay",
  "PausePlay",
  "ResumePlay",
  "StopPlay",
  "NextPlayStep",
  "PlayActive",
  "PlayStatus",
  "RefreshGraphics",
  "ClipboardGet",
  "ClipboardSet",
  "GetKudos",
  "SetKudos",
  "AddKudosFavorite",
  "AddKudosQueue",
  "GetPersistentData",
  "ExportPersistentData",
  "ClearPersistentData",
  "ListNotes",
  "GetNote",
  "SaveNote",
  "DeleteNote",
  "GetWikiSections",
  "GetMapSections",
  "OpenURL",
  "OpenWikiSlug",
  "CalcRankBonus",
  "CalcTrainCost",
  "CheckForUpdate",
  ...Object.keys(settingsOperations),
]);

function settingPayload(method: string, args: any[]): unknown {
  if (method === "SetCRTEffects") {
    return { scanlines: args[0], roll: args[1], bloom: args[2] };
  }
  return args[0];
}
