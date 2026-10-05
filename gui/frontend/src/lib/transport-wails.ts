import type { PraetorTransport, TransportHandlers } from "./transport";

export class WailsTransport implements PraetorTransport {
  readonly kind = "wails" as const;

  private app(): Record<string, (...a: any[]) => Promise<any>> | undefined {
    return window.go?.gui?.GuiApp;
  }

  async invoke<T>(method: string, fallback: T, ...args: any[]): Promise<T> {
    const app = this.app();
    if (!app || typeof app[method] !== "function") return fallback;
    try {
      return (await app[method](...args)) as T;
    } catch (error) {
      console.error(`GuiApp.${method} failed:`, error);
      throw error;
    }
  }

  subscribe(handlers: TransportHandlers): () => void {
    if (!window.runtime) return () => {};
    return window.runtime.EventsOn("praetor:events", (data: any[]) => handlers.events(data));
  }

  async start(): Promise<void> {
    await this.invoke<void>("Start", undefined);
  }

  async webLogin(_password: string): Promise<void> {}
  async webLogout(): Promise<void> {}
  async quit(): Promise<void> {
    window.runtime?.Quit();
  }
  showLocalNotification(_title: string, _message: string): void {}
  async requestNotificationPermission(): Promise<"unsupported"> {
    return "unsupported";
  }
}
