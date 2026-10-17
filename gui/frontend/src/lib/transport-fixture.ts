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
        credentialStore: {
          backend: "memory",
          available: true,
          canStore: true,
        },
        hasModes: true,
        modeNames: ["fixture"],
        activeCharacterProfileId: "b7782d7d-5e5d-54d7-8c66-3e3c9a3b1f50",
        config: {
          CharacterProfiles: {
            DefaultID: "b7782d7d-5e5d-54d7-8c66-3e3c9a3b1f50",
            Profiles: [{
              ID: "b7782d7d-5e5d-54d7-8c66-3e3c9a3b1f50",
              Name: "Default",
              Variables: {},
              CustomViews: [],
              ActionSets: [],
              SelectedActionSetID: "",
            }],
          },
          DisplayProfiles: {
            DefaultID: "0b40c7dd-fba7-44cb-83f7-f125f5bc98b3",
            Profiles: [{
              ID: "0b40c7dd-fba7-44cb-83f7-f125f5bc98b3",
              Name: "Default",
              SidebarsVisible: true,
              Main: [{ ComponentID: "a1ebe82e-7f0d-4ba9-9e00-181d045e3e12", SidebarHeight: 0, Collapsed: false }],
              Sidebars: [],
            }],
          },
          UI: {
            DisplayMode: "sidebar",
            Scrollback: 5000,
          },
        } as unknown as AppConfig,
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
