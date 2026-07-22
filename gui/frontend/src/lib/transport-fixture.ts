import type { AppConfig } from "./types";
import type { PraetorTransport, TransportHandlers } from "./transport";
import { webFixtureEvents } from "./web-fixture";

// Deterministic transport for component/parity tests. It exercises the same
// snapshot callback used by web reconnects without Wails, HTTP, or live TEC.
export class FixtureTransport implements PraetorTransport {
  readonly kind = "web" as const;
  private handlers = new Set<TransportHandlers>();

  async invoke<T>(method: string, fallback: T): Promise<T> {
    if (method === "GetInitState") {
      return {
        version: "fixture",
        debug: true,
        accounts: ["fixture-account"],
        hasModes: true,
        modeNames: ["fixture"],
        config: { UI: { DisplayMode: "sidebar", Scrollback: 5000, CustomTabs: [] } } as unknown as AppConfig,
      } as T;
    }
    return fallback;
  }

  subscribe(handlers: TransportHandlers): () => void {
    this.handlers.add(handlers);
    return () => this.handlers.delete(handlers);
  }

  async start(): Promise<void> {
    for (const handlers of this.handlers) handlers.snapshot?.(webFixtureEvents);
  }

  async webLogin(_password: string): Promise<void> {}
  async webLogout(): Promise<void> {}
  async quit(): Promise<void> {}
  showLocalNotification(_title: string, _message: string): void {}
  async requestNotificationPermission(): Promise<"unsupported"> {
    return "unsupported";
  }
}
