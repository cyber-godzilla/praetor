// Typed wrapper around the Wails GuiApp bindings and runtime events. When the
// app runs outside Wails (e.g. `vite dev` in a plain browser), window.go is
// undefined; calls resolve to safe defaults so the UI still renders for
// layout work.

import type {
  InitState,
  AppConfig,
  KudosConfig,
  PersistentKeyInfo,
  WikiSection,
  RBResult,
  WireEvent,
  HighlightConfig,
  CustomTabConfig,
  ActionSet,
  DesktopNotificationsConfig,
  UpdateInfo,
  NoteSummary,
  Note,
  SendPreview,
  PlayPreview,
  PlayState,
  AccountState,
  ConnectResult,
} from "./types";

function app(): Record<string, (...a: any[]) => Promise<any>> | undefined {
  return window.go?.gui?.GuiApp;
}

export function inWails(): boolean {
  return !!window.go?.gui?.GuiApp && !!window.runtime;
}

async function call<T>(method: string, fallback: T, ...args: any[]): Promise<T> {
  const a = app();
  if (!a || typeof a[method] !== "function") {
    return fallback;
  }
  try {
    return (await a[method](...args)) as T;
  } catch (e) {
    console.error(`GuiApp.${method} failed:`, e);
    throw e;
  }
}

// ---- Lifecycle & init ----
export const getInitState = () =>
  call<InitState>("GetInitState", {
    version: "dev",
    debug: false,
    accounts: [],
    credentialStore: {
      backend: "unconfigured",
      available: false,
      canStore: false,
      message: "Secure credential storage is not configured.",
    },
    hasModes: false,
    modeNames: [],
    config: {} as AppConfig,
  });

export const getConfig = () => call<AppConfig>("GetConfig", {} as AppConfig);
export const start = () => call<void>("Start", undefined);

// ---- Auth / connection ----
export const listAccounts = () => call<AccountState>("ListAccounts", {
  accounts: [],
  credentialStore: {
    backend: "unconfigured",
    available: false,
    canStore: false,
    message: "Secure credential storage is not configured.",
  },
});
export const connectNew = (u: string, p: string, store: boolean) =>
  call<ConnectResult>("ConnectNew", {
    connected: false,
    credentialSaveRequested: store,
    credentialsSaved: false,
  }, u, p, store);
export const connectStored = (u: string) => call<void>("ConnectStored", undefined, u);
export const saveAccount = (u: string, p: string) => call<void>("SaveAccount", undefined, u, p);
export const removeAccount = (u: string) => call<void>("RemoveAccount", undefined, u);
export const disconnect = () => call<void>("Disconnect", undefined);

// ---- Input / modes ----
export const send = (input: string) => call<void>("Send", undefined, input);
export const modeNames = () => call<string[]>("ModeNames", []);
export const currentMode = () => call<string>("CurrentMode", "");
export const setMode = (name: string, args: string[]) =>
  call<void>("SetMode", undefined, name, args);
export const reloadScripts = () => call<void>("ReloadScripts", undefined);

// ---- Graphics ----
export const refreshGraphics = () => call<void>("RefreshGraphics", undefined);

// ---- Clipboard ----
export const clipboardGet = () => call<string>("ClipboardGet", "");
export const clipboardSet = (t: string) => call<void>("ClipboardSet", undefined, t);

// ---- Settings toggles ----
export const setEchoTyped = (v: boolean) => call<void>("SetEchoTyped", undefined, v);
export const setEchoScript = (v: boolean) => call<void>("SetEchoScript", undefined, v);
export const setColorWords = (v: boolean) => call<void>("SetColorWords", undefined, v);
export const setHideIPs = (v: boolean) => call<void>("SetHideIPs", undefined, v);
export const setInputSpellcheck = (v: boolean) => call<void>("SetInputSpellcheck", undefined, v);
export const setUpdateCheck = (v: boolean) => call<void>("SetUpdateCheck", undefined, v);
export const setRetainAppLogs = (v: boolean) => call<void>("SetRetainAppLogs", undefined, v);
export const setSessionLogging = (v: boolean) => call<void>("SetSessionLogging", undefined, v);
export const setLogPath = (p: string) => call<void>("SetLogPath", undefined, p);
export const setDisplayMode = (m: string) => call<void>("SetDisplayMode", undefined, m);
export const setNumpadNavigation = (m: string) => call<void>("SetNumpadNavigation", undefined, m);
export const setMinimapScale = (s: number) => call<void>("SetMinimapScale", undefined, s);
export const setCompassScale = (s: number) => call<void>("SetCompassScale", undefined, s);
export const setOutputFontSize = (px: number) => call<void>("SetOutputFontSize", undefined, px);
export const setCRTEffects = (scanlines: boolean, roll: boolean, bloom: boolean) =>
  call<void>("SetCRTEffects", undefined, scanlines, roll, bloom);

// ---- Settings lists ----
export const setHighlights = (h: HighlightConfig[]) => call<void>("SetHighlights", undefined, h);
export const setCustomTabs = (t: CustomTabConfig[]) => call<void>("SetCustomTabs", undefined, t);
export const setActionSets = (s: ActionSet[]) => call<void>("SetActionSets", undefined, s);
export const setQuickCycleModes = (m: string[]) => call<void>("SetQuickCycleModes", undefined, m);
export const setHighPriority = (c: string[]) => call<void>("SetHighPriority", undefined, c);
export const setIgnoreOOC = (n: string[]) => call<void>("SetIgnoreOOC", undefined, n);
export const setIgnoreThink = (n: string[]) => call<void>("SetIgnoreThink", undefined, n);
export const setScriptDirs = (d: string[]) => call<void>("SetScriptDirs", undefined, d);
export const pickScriptDir = () => call<string>("PickScriptDir", "");
export const setNotifications = (c: DesktopNotificationsConfig) =>
  call<void>("SetNotifications", undefined, c);

// ---- Kudos ----
export const getKudos = () =>
  call<KudosConfig>("GetKudos", { Favorites: [], Queue: [] });
export const setKudos = (k: KudosConfig) => call<void>("SetKudos", undefined, k);
export const addKudosFavorite = (name: string) => call<boolean>("AddKudosFavorite", false, name);
export const addKudosQueue = (name: string, msg: string) =>
  call<void>("AddKudosQueue", undefined, name, msg);

// ---- Persistent data ----
export const getPersistentData = () => call<PersistentKeyInfo[]>("GetPersistentData", []);
export const exportPersistentData = (keys: string[]) =>
  call<string>("ExportPersistentData", "", keys);
export const clearPersistentData = (keys: string[]) =>
  call<void>("ClearPersistentData", undefined, keys);

// ---- Wiki / maps / calc ----
export const getWikiSections = () => call<WikiSection[]>("GetWikiSections", []);
export const getMapSections = () => call<WikiSection[]>("GetMapSections", []);
export const openURL = (url: string) => call<void>("OpenURL", undefined, url);
export const openWikiSlug = (slug: string) => call<void>("OpenWikiSlug", undefined, slug);
export const calcRankBonus = (mode: number, basics: number, subskill: number) =>
  call<RBResult>("CalcRankBonus", { mode, basics, subskill, basicsRB: 0, subskillRB: 0, cells: [] }, mode, basics, subskill);
export const calcTrainCost = (
  curRank: number,
  desRank: number,
  slot: number,
  difficulty: number,
  selfTrained: boolean,
  selfTaught: boolean,
  healing: boolean,
) => call<number>("CalcTrainCost", 0, curRank, desRank, slot, difficulty, selfTrained, selfTaught, healing);

// ---- Updates ----
export const checkForUpdate = () =>
  call<UpdateInfo>("CheckForUpdate", { available: false, current: "", latest: "", url: "" });

// ---- Notes ----
export const listNotes = () => call<NoteSummary[]>("ListNotes", []);
export const getNote = (title: string) =>
  call<Note>("GetNote", { title: "", body: "" }, title);
export const saveNote = (originalTitle: string, title: string, body: string) =>
  call<void>("SaveNote", undefined, originalTitle, title, body);
export const deleteNote = (title: string) => call<void>("DeleteNote", undefined, title);

// ---- Multi-line send ----
export const pickSendFile = () =>
  call<SendPreview>("PickSendFile", { path: "", name: "", lines: 0, batches: 0 });
export const startFileSend = (path: string) => call<void>("StartFileSend", undefined, path);
export const abortSend = () => call<boolean>("AbortSend", false);

// ---- Play ----
export const pickPlayFile = () =>
  call<PlayPreview>("PickPlayFile", { path: "", name: "", steps: 0, fixedMs: 0, hasCues: false, errors: [] });
export const startPlay = (path: string) => call<void>("StartPlay", undefined, path);
export const pausePlay = () => call<boolean>("PausePlay", false);
export const resumePlay = () => call<boolean>("ResumePlay", false);
export const stopPlay = () => call<boolean>("StopPlay", false);
export const nextPlayStep = () => call<boolean>("NextPlayStep", false);
export const playActive = () => call<boolean>("PlayActive", false);
export const playStatus = () =>
  call<PlayState>("PlayStatus", { active: false, paused: false, step: 0, total: 0 });

// ---- Events ----
export function onEvents(cb: (batch: WireEvent[]) => void): () => void {
  if (!window.runtime) return () => {};
  return window.runtime.EventsOn("praetor:events", (data: WireEvent[]) => cb(data));
}
