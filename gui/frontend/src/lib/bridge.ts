// Transport-neutral API used by the shared Svelte frontend. The native Wails
// shell and the authenticated browser shell implement the same operation set.

import type {
  InitState,
  ModeSpec,
  AppConfig,
  KudosConfig,
  PersistentKeyInfo,
  WikiSection,
  RBResult,
  TrainingCostRow,
  WireEvent,
  HighlightConfig,
  CustomTabConfig,
  ActionSet,
  DisplayProfileConfig,
  NotificationsConfig,
  UpdateInfo,
  NoteSummary,
  Note,
  SendPreview,
  PlayPreview,
  PlayState,
  AccountState,
  ConnectResult,
  InputChainStatus,
  SpeechColorConfig,
  WoundHighlightConfig,
  CharacterProfileIdentity,
  ScriptManagerState,
  ManagedScriptSourceConfig,
  ManagedScriptUpdateStatus,
} from "./types";
import type { PraetorTransport, SystemUpdate } from "./transport";
import { WebAuthRequiredError } from "./transport";
import { WailsTransport } from "./transport-wails";
import { WebTransport } from "./transport-web";
import { layoutWithIntegerPixels } from "./layout";

const transport: PraetorTransport = window.go?.gui?.GuiApp && window.runtime
  ? new WailsTransport()
  : new WebTransport();

const call = <T>(method: string, fallback: T, ...args: any[]) =>
  transport.invoke<T>(method, fallback, ...args);

export { WebAuthRequiredError };

export function inWails(): boolean {
  return transport.kind === "wails";
}

export function inWeb(): boolean {
  return transport.kind === "web";
}

export const webLogin = (password: string) => transport.webLogin(password);
export const webLogout = () => transport.webLogout();
export const quit = () => transport.quit();
export const showLocalNotification = (title: string, message: string, sound = false) =>
  transport.showLocalNotification(title, message, sound);
export const setWindowFocused = (focused: boolean) => call<void>("SetWindowFocused", undefined, focused);
export const requestNotificationPermission = () => transport.requestNotificationPermission();

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
    modeSpecs: [],
    colorwordStyles: [],
    activeCharacterProfileId: "",
    config: {} as AppConfig,
  });

export const getConfig = () => call<AppConfig>("GetConfig", {} as AppConfig);
export const start = () => transport.start();

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
export const removeAccount = (u: string) => call<void>("RemoveAccount", undefined, u);
export const disconnect = () => call<void>("Disconnect", undefined);

// ---- Input / modes ----
export const send = (input: string) => call<void>("Send", undefined, input);
export const sendInput = (input: string) => call<void>("SendInput", undefined, input);
export const inputChainActive = () => call<boolean>("InputChainActive", false);
export const inputChainStatus = () => call<InputChainStatus>("InputChainStatus", {
  active: false,
  chains: 0,
  step: 0,
  total: 0,
  state: "",
});
export const abortInputChains = () => call<number>("AbortInputChains", 0);
export const modeNames = () => call<string[]>("ModeNames", []);
export const modeSpecs = () => call<ModeSpec[]>("ModeSpecs", []);
export const setMode = (name: string, args: string[]) =>
  call<void>("SetMode", undefined, name, args);
export const reloadScripts = () => call<void>("ReloadScripts", undefined);
export const getScriptManager = () => call<ScriptManagerState>("GetScriptManager", {
  localDirectories: [], managed: [], suggestedURL: "", suggestedInstalled: false,
});
export const installManagedScript = (url: string) =>
  call<ManagedScriptSourceConfig>("InstallManagedScript", {} as ManagedScriptSourceConfig, url);
export const checkManagedScript = (id: string) =>
  call<ManagedScriptUpdateStatus>("CheckManagedScript", {
    available: false, updated: false, version: "", commit: "", tracking: "", ref: "",
  }, id);
export const updateManagedScript = (id: string) =>
  call<ManagedScriptSourceConfig>("UpdateManagedScript", {} as ManagedScriptSourceConfig, id);
export const removeManagedScript = (id: string) =>
  call<void>("RemoveManagedScript", undefined, id);
export const setManagedScriptStartupCheck = (id: string, enabled: boolean) =>
  call<void>("SetManagedScriptStartupCheck", undefined, id, enabled);
export const setManagedScriptEnabled = (id: string, enabled: boolean, stopAutomation = false) =>
  call<void>("SetManagedScriptEnabled", undefined, id, enabled, stopAutomation);
export const setLocalScriptEnabled = (path: string, enabled: boolean, stopAutomation = false) =>
  call<void>("SetLocalScriptEnabled", undefined, path, enabled, stopAutomation);
export const setManagedScriptTrust = (id: string, trust: "restricted" | "trusted", stopAutomation = false) =>
  call<void>("SetManagedScriptTrust", undefined, id, trust, stopAutomation);
export const reorderManagedScripts = (ids: string[]) =>
  call<void>("ReorderManagedScripts", undefined, ids);

// ---- Clipboard ----
export const clipboardGet = () => call<string>("ClipboardGet", "");
export const clipboardSet = (t: string) => call<void>("ClipboardSet", undefined, t);

// ---- Settings toggles ----
export const setEchoTyped = (v: boolean) => call<void>("SetEchoTyped", undefined, v);
export const setEchoScript = (v: boolean) => call<void>("SetEchoScript", undefined, v);
export const setColorWords = (v: boolean) => call<void>("SetColorWords", undefined, v);
export const setHideIPs = (v: boolean) => call<void>("SetHideIPs", undefined, v);
export const setInputSpellcheck = (v: boolean) => call<void>("SetInputSpellcheck", undefined, v);
export const setKeepInputOnSend = (v: boolean) => call<void>("SetKeepInputOnSend", undefined, v);
export const setSemicolonDelay = (ms: number) => call<void>("SetSemicolonDelay", undefined, ms);
export const setUnbusyDelay = (ms: number) => call<void>("SetUnbusyDelay", undefined, ms);
export const setUpdateCheck = (v: boolean) => call<void>("SetUpdateCheck", undefined, v);
export const setMobileShowToolbar = (v: boolean) =>
  call<void>("SetMobileShowToolbar", undefined, v);
export const setMobileShowTabBar = (v: boolean) =>
  call<void>("SetMobileShowTabBar", undefined, v);
export const setMobileHideNavigationOnInput = (v: boolean) =>
  call<void>("SetMobileHideNavigationOnInput", undefined, v);
export const setMobileLowercaseFirstLetter = (v: boolean) =>
  call<void>("SetMobileLowercaseFirstLetter", undefined, v);
export const setMobileOutputFontSize = (px: number) =>
  call<void>("SetMobileOutputFontSize", undefined, px);
export const setRetainAppLogs = (v: boolean) => call<void>("SetRetainAppLogs", undefined, v);
export const setSessionLogging = (v: boolean) => call<void>("SetSessionLogging", undefined, v);
export const setLogPath = (p: string) => call<void>("SetLogPath", undefined, p);
export const setDisplayMode = (m: string) => call<void>("SetDisplayMode", undefined, m);
export const setGUISidebarsVisible = (visible: boolean) =>
  call<void>("SetGUISidebarsVisible", undefined, visible);
export const setNumpadNavigation = (m: string) => call<void>("SetNumpadNavigation", undefined, m);
export const setMinimapScale = (s: number) => call<void>("SetMinimapScale", undefined, s);
export const setCompassScale = (s: number) => call<void>("SetCompassScale", undefined, s);
export const setOutputFontSize = (px: number) => call<void>("SetOutputFontSize", undefined, px);
export const setGUILayout = (sidebarWidth: number, minimapHeight: number) =>
  call<void>("SetGUILayout", undefined, sidebarWidth, minimapHeight);
export const setCRTEffects = (scanlines: boolean, roll: boolean, bloom: boolean) =>
  call<void>("SetCRTEffects", undefined, scanlines, roll, bloom);

// ---- Settings lists ----
export const setHighlights = (h: HighlightConfig[]) => call<void>("SetHighlights", undefined, h);
export const setSpeechColors = (rules: SpeechColorConfig[]) =>
  call<void>("SetSpeechColors", undefined, rules);
export const setWoundHighlights = (config: WoundHighlightConfig) =>
  call<void>("SetWoundHighlights", undefined, config);
export const setCustomTabs = (t: CustomTabConfig[]) => call<void>("SetCustomTabs", undefined, t);
export const setViewLayout = (layout: DisplayProfileConfig, customViews: CustomTabConfig[]) =>
  call<void>("SetViewLayout", undefined, layoutWithIntegerPixels(layout), customViews);
export const setProfileViewLayout = (profileID: string, layout: DisplayProfileConfig, customViews: CustomTabConfig[]) =>
  call<void>("SetProfileViewLayout", undefined, profileID, layoutWithIntegerPixels(layout), customViews);
export const setActionSets = (s: ActionSet[]) => call<void>("SetActionSets", undefined, s);
export const setActionSetInheritance = (inherit: boolean) =>
  call<void>("SetActionSetInheritance", undefined, inherit);
export const deleteActionSetEverywhere = (id: string) =>
  call<void>("DeleteActionSetEverywhere", undefined, id);
export const setSelectedActionSetID = (id: string) =>
  call<void>("SetSelectedActionSetID", undefined, id);
export const setQuickCycleModes = (m: string[]) => call<void>("SetQuickCycleModes", undefined, m);
export const setQuickCycleInheritance = (inherit: boolean) =>
  call<void>("SetQuickCycleInheritance", undefined, inherit);
export const setHighPriority = (c: string[]) => call<void>("SetHighPriority", undefined, c);
export const setHighPriorityInheritance = (inherit: boolean) =>
  call<void>("SetHighPriorityInheritance", undefined, inherit);
export const setInputVariables = (v: Record<string, string>) =>
  call<void>("SetInputVariables", undefined, v);
export const setCharacterProfiles = (profiles: CharacterProfileIdentity[]) =>
  call<void>("SetCharacterProfiles", undefined, profiles);
export const setCharacterProfileAutoSwitch = (enabled: boolean) =>
  call<void>("SetCharacterProfileAutoSwitch", undefined, enabled);
export const switchCharacterProfile = (id: string, stopAutomation = false) =>
  call<void>("SwitchCharacterProfile", undefined, id, stopAutomation);
export const setIgnoreOOC = (n: string[]) => call<void>("SetIgnoreOOC", undefined, n);
export const setIgnoreThink = (n: string[]) => call<void>("SetIgnoreThink", undefined, n);
export const setScriptDirs = (d: string[]) => call<void>("SetScriptDirs", undefined, d);
export const pickScriptDir = () => call<string>("PickScriptDir", "");
export const setNotifications = (c: NotificationsConfig) =>
  call<void>("SetNotifications", undefined, c);
export const testDiscordNotification = () =>
  call<void>("TestDiscordNotification", undefined);

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
export const calcTrainingCosts = (
  curBasics: number,
  curSub: number,
  tgtBasics: number,
  tgtSub: number,
  selfTrained: boolean,
  selfTaught: boolean,
  healing: boolean,
) => call<TrainingCostRow[]>(
  "CalcTrainingCosts",
  [],
  curBasics,
  curSub,
  tgtBasics,
  tgtSub,
  selfTrained,
  selfTaught,
  healing,
);

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
export function onEvents(
  events: (batch: WireEvent[]) => void,
  snapshot?: (batch: WireEvent[]) => void,
  system?: (update: SystemUpdate) => void,
): () => void {
  return transport.subscribe({ events, snapshot, system });
}
