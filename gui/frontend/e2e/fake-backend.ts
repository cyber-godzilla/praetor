import { expect, type Locator, type Page } from "@playwright/test";
import type { InitState, InputChainStatus, TextPayload, WireEvent } from "../src/lib/types";

export interface FakeCall {
  method: string;
  args: unknown[];
}

// Hooks the init script leaves on window for the test side.
interface FakeHooks {
  emit(event: string, data: unknown): void;
  calls(method?: string): FakeCall[];
  listeners(event: string): number;
  setReader(method: string, value: unknown): void;
  setInputChainStatus(status: InputChainStatus): void;
}

declare global {
  interface Window {
    __praetorFake?: FakeHooks;
  }
}

export interface FakeBackend {
  boot(): Promise<void>;
  connect(): Promise<void>;
  enterGame(): Promise<void>;
  events(batch: WireEvent[]): Promise<void>;
  text(items: (string | TextPayload)[]): Promise<void>;
  calls(method?: string): Promise<FakeCall[]>;
  args(method: string): Promise<unknown[][]>;
  setReader(method: string, value: unknown): Promise<void>;
  setInputChainStatus(status: InputChainStatus): Promise<void>;
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
    let inputChainActive = false;
    let inputChainStatus: InputChainStatus = { active: false, chains: 0, step: 0, total: 0, state: "" };
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
      GetScriptManager: {
        localDirectories: init.config.Scripts ?? [],
        managed: [],
        suggestedURL: "https://github.com/cyber-godzilla/praetor-scripts.git",
        suggestedInstalled: false,
      },
      AddKudosFavorite: false,
      // src/lib/bridge.ts declares this returning a string, not void.
      ExportPersistentData: "",
    };
    const writers = new Set([
      "Start", "Send", "SendInput", "SetMode", "RemoveAccount", "Disconnect",
      "ReloadScripts", "ClipboardSet", "OpenURL", "OpenWikiSlug",
      "SaveNote", "DeleteNote", "StartFileSend", "StartPlay", "ClearPersistentData",
	  "AddKudosQueue", "TestDiscordNotification",
    ]);
    const connectors = new Set(["ConnectNew", "ConnectStored"]);

    const guiApp = new Proxy({} as Record<string, (...a: unknown[]) => Promise<unknown>>, {
      get(_target, name) {
        if (typeof name !== "string") return undefined;
        // The Proxy is awaited transitively (e.g. `await guiApp` in some call
        // site) — without this, `then`/`catch`/`finally` resolve to functions
        // that don't behave like a thenable's, and anything awaiting the
        // object itself (not a call result) hangs forever.
        if (name === "then" || name === "catch" || name === "finally") return undefined;
        return (...args: unknown[]) => {
          calls.push({ method: name, args });
          if (name === "InputChainActive") return Promise.resolve(inputChainActive);
          if (name === "InputChainStatus") return Promise.resolve(inputChainStatus);
          if (name === "AbortInputChains") {
            const canceled = inputChainActive ? 1 : 0;
            inputChainActive = false;
            inputChainStatus = { active: false, chains: 0, step: 0, total: 0, state: "" };
            return Promise.resolve(canceled);
          }
          if (name === "SendInput") {
            const input = typeof args[0] === "string" ? args[0] : "";
            if (input.trim().toLowerCase() === "/character") {
              setTimeout(() => emit("praetor:events", [{
                kind: "openMenu",
                openMenu: "menu-character",
              }]), 0);
            }
            if (input.includes(";;") || input.includes("&&") || input.includes("$(")) {
              inputChainActive = true;
              inputChainStatus = {
                active: true,
                chains: 1,
                step: input.includes("&&") || input.includes(";;") ? 2 : 1,
                total: input.includes("&&") || input.includes(";;") ? 2 : 1,
                state: input.includes("wait-for") ? "wait-for" : input.includes("&&") ? "unbusy" : "pacing",
                ...(input.includes("wait-for") ? { detail: "ready", durationMs: 30000, remainingMs: 30000 } : {}),
                ...(input.includes(";;") && !input.includes("wait-for") ? { durationMs: 900, remainingMs: 899 } : {}),
              };
            }
          }
          if (name === "SetCharacterProfiles") {
            const identities = (args[0] ?? []) as Array<{
              ID: string; Name: string; Account: string; CharacterNumber: number;
            }>;
            const profiles = init.config.CharacterProfiles.Profiles;
            const byID = new Map(profiles.map((profile) => [profile.ID, profile]));
            const fallback = profiles.find((profile) => profile.ID === init.config.CharacterProfiles.DefaultID)!;
            init.config.CharacterProfiles.Profiles = identities.map((identity) => {
              const current = byID.get(identity.ID);
              if (current) return { ...current, ...identity };
              return {
                ...structuredClone(fallback),
                ...identity,
                Variables: {},
                ActionSetsMode: "inherit",
                ActionSets: [],
                QuickCycleMode: "inherit",
                QuickCycleModes: null,
                PriorityCommandsMode: "inherit",
                HighPriority: null,
                CustomViews: (fallback.CustomViews ?? []).map((view) => ({ ...view, ID: crypto.randomUUID() })),
              };
            });
            const display = init.config.DisplayProfiles;
            const defaultLayout = display.Profiles.find((profile) => profile.ID === display.DefaultID)!;
            const layoutByCharacter = new Map(
              display.Profiles
                .filter((profile) => profile.CharacterProfileID)
                .map((profile) => [profile.CharacterProfileID!, profile]),
            );
            display.Profiles = [defaultLayout, ...init.config.CharacterProfiles.Profiles
              .filter((profile) => profile.ID !== init.config.CharacterProfiles.DefaultID)
              .map((profile) => {
                const current = layoutByCharacter.get(profile.ID);
                if (current) return { ...current, Name: profile.Name };
                const inherited = structuredClone(defaultLayout);
                inherited.ID = crypto.randomUUID();
                inherited.Name = profile.Name;
                inherited.CharacterProfileID = profile.ID;
                const customIDs = new Map(
                  (fallback.CustomViews ?? []).map((view, index) => [view.ID, profile.CustomViews?.[index]?.ID]),
                );
                const remap = (components: typeof inherited.Main) => components.map((component) => ({
                  ...component,
                  ComponentID: customIDs.get(component.ComponentID) ?? component.ComponentID,
                }));
                inherited.Main = remap(inherited.Main);
                inherited.Bench = remap(inherited.Bench).filter((component) =>
                  !fallback.CustomViews?.some((view) => view.ID === component.ComponentID),
                );
                inherited.Sidebars = inherited.Sidebars.map((sidebar) => ({
                  ...sidebar,
                  ID: crypto.randomUUID(),
                  Components: remap(sidebar.Components),
                }));
                return inherited;
              })];
            readers.GetConfig = init.config;
          }
          if (name === "SetCharacterProfileAutoSwitch") {
            init.config.CharacterProfiles.AutoSwitch = Boolean(args[0]);
            readers.GetConfig = init.config;
          }
          if (name === "SwitchCharacterProfile") {
            const profile = init.config.CharacterProfiles.Profiles.find((candidate) => candidate.ID === args[0]);
            if (profile) {
              setTimeout(() => emit("praetor:events", [{
                kind: "characterProfile",
                characterProfile: { id: profile.ID, name: profile.Name },
              }]), 0);
            }
            return Promise.resolve(undefined);
          }
          if (name === "CalcRankBonus") {
            const [mode, basics, subskill] = args as number[];
            const fixtureKey = `${name}:${mode}:${basics}:${subskill}`;
            if (Object.hasOwn(readers, fixtureKey)) {
              return Promise.resolve(readers[fixtureKey]);
            }
            return Promise.resolve({
              mode,
              basics,
              subskill,
              basicsRB: basics / 10,
              subskillRB: subskill / 10,
              cells: Array.from({ length: 25 }, (_, i) => ({
                posture: Math.floor(i / 5),
                difficulty: i % 5,
                bonus: basics + subskill + i,
              })),
            });
          }
          if (name === "CalcTrainingCosts") {
            const fixtureKey = `${name}:${args.join(":")}`;
            if (Object.hasOwn(readers, fixtureKey)) {
              return Promise.resolve(readers[fixtureKey]);
            }
            return Promise.resolve(Array.from({ length: 20 }, (_, i) => ({
              slot: i + 1,
              basic: (i + 1) * 10,
              easy: (i + 1) * 20,
              average: (i + 1) * 30,
              difficult: (i + 1) * 40,
              impossible: (i + 1) * 50,
            })));
          }
          if (Object.hasOwn(readers, name)) return Promise.resolve(readers[name]);
          if (connectors.has(name)) {
            // Next macrotask, like a real async connect; the real store then
            // runs its own screen transition on the conn event.
            setTimeout(() => emit("praetor:events", [{ kind: "conn", conn: { state: "connected" } }]), 0);
            return Promise.resolve(undefined);
          }
          // Every Go `Set*` binding returns void. A `Set*` binding that
          // returns a value must be added to `readers` above, or it will
          // silently resolve `undefined` here instead of its real value.
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
      setReader: (method, value) => { readers[method] = value; },
      setInputChainStatus: (status) => {
        inputChainStatus = status;
        inputChainActive = status.active;
      },
    };
    // test.ts fails a test on this console line, same as an unhandled GuiApp
    // call — a swallowed rejection in the app would otherwise pass silently.
    window.addEventListener("unhandledrejection", (e) =>
      console.error("fake-backend: unhandledrejection " + String(e.reason)),
    );
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
    async enterGame() {
      await events([{ kind: "conn", conn: { state: "connected" } }]);
      await expect(input).toBeVisible();
    },
    async connect() {
      await backend.enterGame();
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
    async calls(method) {
      return page.evaluate((m) => window.__praetorFake!.calls(m), method);
    },
    async args(method) {
      return (await backend.calls(method)).map((c) => c.args);
    },
    async setReader(method, value) {
      await page.evaluate(
        ([readerMethod, readerValue]) => window.__praetorFake!.setReader(readerMethod, readerValue),
        [method, value] as const,
      );
    },
    async setInputChainStatus(status) {
      await page.evaluate(
        (nextStatus) => window.__praetorFake!.setInputChainStatus(nextStatus),
        status,
      );
    },
  };
  return backend;
}
