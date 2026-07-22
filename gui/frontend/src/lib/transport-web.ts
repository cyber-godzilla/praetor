import type {
  AppConfig,
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
  result?: { operation: string; ok: boolean; message?: string };
}

interface ErrorResponse {
  error?: { code?: string; message?: string; requestId?: string };
}

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
        const data = await this.request<{ accounts: string[] }>("GET", "/api/v1/accounts");
        return (data.accounts ?? []) as T;
      }
      case "ConnectNew":
        await this.request("POST", "/api/v1/game/connect", {
          username: args[0],
          password: args[1],
          store: args[2],
        });
        return undefined as T;
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
    this.reconnectAttempt = 0;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
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
      this.started = false;
      if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
      this.socket?.close(1000, "signed out");
      this.socket = null;
      this.csrf = "";
      this.serverId = "";
      this.sequence = 0;
      this.socketReady = false;
      this.emitSystem({ type: "auth-expired" });
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
    if (init.protocol !== 1) throw new Error(`Unsupported Praetor web protocol ${init.protocol}`);
    this.csrf = init.csrf;
    this.revision = init.configRevision;
    this.serverId = init.serverId;
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
      if (this.socket === socket) this.socket = null;
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
      if (message.accounts) this.emitSystem({ type: "accounts", accounts: message.accounts });
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
      this.emitSystem({ type: "accounts", accounts: message.accounts ?? [] });
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
          this.started = false;
          this.emitSystem({ type: "auth-expired" });
        } else if (this.started) {
          this.scheduleReconnect();
        }
      }
    }, delay);
  }

  private async request<T = unknown>(method: string, url: string, body?: unknown, authenticated = true): Promise<T> {
    if (
      authenticated &&
      method !== "GET" &&
      method !== "HEAD" &&
      url !== "/api/v1/auth/logout" &&
      this.started &&
      !this.socketReady
    ) {
      throw new Error("Praetor is reconnecting; wait for current state before making changes.");
    }
    const headers: Record<string, string> = { Accept: "application/json" };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (authenticated && method !== "GET" && method !== "HEAD") headers["X-Praetor-CSRF"] = this.csrf;
    const response = await fetch(url, {
      method,
      headers,
      credentials: "same-origin",
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (response.status === 401) throw new WebAuthRequiredError();
    if (!response.ok) {
      let detail: ErrorResponse = {};
      try { detail = await response.json(); } catch { /* use status fallback */ }
      throw new WebAPIError(
        response.status,
        detail.error?.code ?? "request_failed",
        detail.error?.message ?? `Request failed (${response.status})`,
      );
    }
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
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
    if (this.started && !this.socketReady) {
      throw new Error("Praetor is reconnecting; wait for current state before exporting data.");
    }
    const response = await fetch("/api/v1/persistent/export", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json", "X-Praetor-CSRF": this.csrf },
      body: JSON.stringify({ keys }),
    });
    if (response.status === 401) throw new WebAuthRequiredError();
    if (!response.ok) throw new Error(`Export failed (${response.status})`);
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

interface ConfigMutationResponse {
  config: AppConfig;
  revision: number;
}

export const settingsOperations: Record<string, string> = {
  SetEchoTyped: "echo-typed",
  SetEchoScript: "echo-script",
  SetColorWords: "color-words",
  SetHideIPs: "hide-ips",
  SetMobileShowToolbar: "mobile-show-toolbar",
  SetMobileShowTabBar: "mobile-show-tab-bar",
  SetMobileHideNavigationOnInput: "mobile-hide-navigation-on-input",
  SetMobileLowercaseFirstLetter: "mobile-lowercase-first-letter",
  SetMobileOutputFontSize: "mobile-output-font-size",
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
  "GetWikiSections",
  "GetMapSections",
  "OpenURL",
  "OpenWikiSlug",
  "CalcRankBonus",
  "CalcTrainCost",
  ...Object.keys(settingsOperations),
]);

function settingPayload(method: string, args: any[]): unknown {
  if (method === "SetCRTEffects") {
    return { scanlines: args[0], roll: args[1], bloom: args[2] };
  }
  return args[0];
}
