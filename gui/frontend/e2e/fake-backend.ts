import { expect, type Locator, type Page } from "@playwright/test";
import type { InitState, TextPayload, WireEvent } from "../src/lib/types";

export interface FakeCall {
  method: string;
  args: unknown[];
}

// Hooks the init script leaves on window for the test side.
interface FakeHooks {
  emit(event: string, data: unknown): void;
  calls(method?: string): FakeCall[];
  listeners(event: string): number;
}

declare global {
  interface Window {
    __praetorFake?: FakeHooks;
  }
}

export interface FakeBackend {
  boot(): Promise<void>;
  connect(): Promise<void>;
  events(batch: WireEvent[]): Promise<void>;
  text(items: (string | TextPayload)[]): Promise<void>;
  status(mode: string): Promise<void>;
  calls(method?: string): Promise<FakeCall[]>;
  args(method: string): Promise<unknown[][]>;
  input: Locator;
  output: Locator;
}

// installFakeBackend installs window.go.gui.GuiApp + window.runtime before the
// page's scripts run, so bridge.ts sees a "real" Wails environment. The init
// function is serialized by Playwright: it must not reference anything outside
// its own body except its `init` argument.
export async function installFakeBackend(page: Page, init: InitState): Promise<FakeBackend> {
  await page.addInitScript((init: InitState) => {
    const listeners = new Map<string, Set<(d: unknown) => void>>();
    const calls: FakeCall[] = [];
    const emit = (event: string, data: unknown) => {
      for (const cb of listeners.get(event) ?? []) cb(data);
    };

    // Readers answer from the fixture. PlayActive/PlayStatus matter: InputLine
    // asks PlayActive before every send.
    const readers: Record<string, unknown> = {
      GetInitState: init,
      GetConfig: init.config,
      ModeNames: init.modeNames ?? [],
      ModeSpecs: init.modeSpecs ?? [],
      CurrentMode: "",
      ListAccounts: init.accounts ?? [],
      ListNotes: [],
      GetWikiSections: [],
      GetMapSections: [],
      GetKudos: { Favorites: [], Queue: [] },
      GetPersistentData: [],
      CheckForUpdate: { available: false, current: init.version, latest: init.version, url: "" },
      PlayActive: false,
      PlayStatus: { active: false, paused: false, step: 0, total: 0 },
      PausePlay: false,
      ResumePlay: false,
      StopPlay: false,
      NextPlayStep: false,
      AbortSend: false,
      ClipboardGet: "",
      PickScriptDir: "",
      AddKudosFavorite: false,
    };
    const writers = new Set([
      "Start", "Send", "SetMode", "SaveAccount", "RemoveAccount", "Disconnect",
      "ReloadScripts", "RefreshGraphics", "ClipboardSet", "OpenURL", "OpenWikiSlug",
      "SaveNote", "DeleteNote", "StartFileSend", "StartPlay", "ClearPersistentData",
      "ExportPersistentData", "AddKudosQueue",
    ]);
    const connectors = new Set(["ConnectNew", "ConnectStored"]);

    const guiApp = new Proxy({} as Record<string, (...a: unknown[]) => Promise<unknown>>, {
      get(_target, name) {
        if (typeof name !== "string") return undefined;
        return (...args: unknown[]) => {
          calls.push({ method: name, args });
          if (name in readers) return Promise.resolve(readers[name]);
          if (connectors.has(name)) {
            // Next macrotask, like a real async connect; the real store then
            // runs its own screen transition on the conn event.
            setTimeout(() => emit("praetor:events", [{ kind: "conn", conn: { state: "connected" } }]), 0);
            return Promise.resolve(undefined);
          }
          if (writers.has(name) || name.startsWith("Set")) return Promise.resolve(undefined);
          console.warn(`fake-backend: unhandled GuiApp.${name}`);
          return Promise.resolve(undefined);
        };
      },
    });

    window.go = { gui: { GuiApp: guiApp } };
    window.runtime = {
      EventsOn(event, cb) {
        if (!listeners.has(event)) listeners.set(event, new Set());
        listeners.get(event)!.add(cb);
        return () => listeners.get(event)?.delete(cb);
      },
      EventsOff(event, ...more) {
        for (const e of [event, ...more]) listeners.delete(e);
      },
      EventsEmit(event, ...data) {
        emit(event, data[0]);
      },
      Quit() {},
      WindowMinimise() {},
      WindowToggleMaximise() {},
    };
    window.__praetorFake = {
      emit,
      calls: (m?: string) => (m ? calls.filter((c) => c.method === m) : calls.slice()),
      listeners: (event) => listeners.get(event)?.size ?? 0,
    };
  }, init);

  const input = page.locator(".inputbar textarea");
  const output = page.getByTestId("e2e-output");

  async function events(batch: WireEvent[]) {
    // App.svelte subscribes after GetInitState resolves; never emit into a void.
    await page.waitForFunction(() => (window.__praetorFake?.listeners("praetor:events") ?? 0) > 0);
    await page.evaluate((b) => window.__praetorFake!.emit("praetor:events", b), batch);
  }

  const backend: FakeBackend = {
    input,
    output,
    async boot() {
      await page.goto("/");
      await expect(page.locator(".splash")).toBeVisible();
      await page.keyboard.press("Space");
      await expect(page.locator(".splash")).toHaveCount(0);
      // Login (no accounts) or account select (accounts) — either is "booted".
      await expect(page.getByText("PRAETOR").first()).toBeVisible();
    },
    async connect() {
      await events([{ kind: "conn", conn: { state: "connected" } }]);
      await expect(input).toBeVisible();
    },
    events,
    async text(items) {
      const batch: WireEvent[] = items.map((it) => {
        const p: TextPayload =
          typeof it === "string" ? { text: it, segments: [{ text: it }], timestamp: 0 } : it;
        return { kind: "text", text: p };
      });
      await events(batch);
    },
    async status(mode) {
      await events([{ kind: "status", status: { mode } }]);
    },
    async calls(method) {
      return page.evaluate((m) => window.__praetorFake!.calls(m), method);
    },
    async args(method) {
      return (await backend.calls(method)).map((c) => c.args);
    },
  };
  return backend;
}
