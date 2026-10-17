import type { AppConfig, ColorwordStyle, InitState, ModeSpec, TextPayload } from "../src/lib/types";

// Everything the fake backend hands the frontend. Keep this the ONE place
// canned state lives so a config-shape change is a one-file fix.

export const VERSION = "v0.0.0-e2e";
export const DEFAULT_CHARACTER_PROFILE_ID = "b7782d7d-5e5d-54d7-8c66-3e3c9a3b1f50";
export const DEFAULT_DISPLAY_PROFILE_ID = "0b40c7dd-fba7-44cb-83f7-f125f5bc98b3";
export const CUSTOM_CHAT_ID = "1fc95886-1541-4db4-8a49-d30391439c65";
export const COMPONENT_IDS = {
  all: "a1ebe82e-7f0d-4ba9-9e00-181d045e3e12",
  metrics: "3888f7f2-16fa-41df-86bc-11c7a2488dbf",
  debug: "a1ba3fe5-10e2-406f-b1f1-7f8746c89762",
  map: "ccb0ec1f-9a08-4d4f-941e-2b8b2c6eb8f1",
  exits: "85178cda-7db8-4fe1-a09d-1a6e572016b2",
  vitals: "e9a0240c-64df-4086-92aa-0f6372f8dde6",
  actions: "fbf29dda-9c5c-4457-8a8b-d5a106206c29",
  modes: "631a65d1-9261-4940-a18b-7ee7bfd48f12",
  variables: "c043ea23-cb1d-4a30-97bd-01e588969d26",
};

export const modeSpecs: ModeSpec[] = [
  { name: "disable", usage: "", desc: "Turn automation off", chains: false },
  { name: "hunt", usage: "[target]", desc: "Hunt the given target", chains: false },
  { name: "hunt_wolves", usage: "", desc: "Hunt wolves specifically", chains: true },
];

export const colorwordStyles: ColorwordStyle[] = [
  { name: "scarlet", label: "Scarlet", family: "Reds", kind: "solid", colors: ["#ff4b3e"] },
  { name: "crimson", label: "Crimson", family: "Reds", kind: "solid", colors: ["#ff3355"] },
  { name: "cinnabar", label: "Cinnabar", family: "Oranges", kind: "solid", colors: ["#ff6037"] },
  { name: "oranges", label: "Oranges", family: "Special", kind: "alternating", colors: ["#ff9a26", "#ff6b1a"] },
  { name: "rainbow", label: "Rainbow", family: "Special", kind: "rainbow", colors: ["#ff0000", "#ff7f00", "#ffff00", "#00ff00", "#0080ff", "#4b0082", "#ee82ee"] },
];

export function makeConfig(): AppConfig {
  return {
    Server: {},
    Commands: { SemicolonDelayMS: 900, UnbusyDelayMS: 100, HighPriority: [] },
    CharacterProfiles: {
      DefaultID: DEFAULT_CHARACTER_PROFILE_ID,
      Profiles: [{
        ID: DEFAULT_CHARACTER_PROFILE_ID,
        Name: "Default",
        Variables: {},
        SelectedActionSetID: "action-combat",
        CustomViews: [
          {
            ID: CUSTOM_CHAT_ID,
            Name: "Chat",
            Visible: true,
            EchoCommands: false,
            Rules: [{ Pattern: "says,", Include: true, Active: true }],
          },
        ],
        ActionSets: [
          {
            ID: "action-combat",
            Name: "Combat",
            Buttons: [
              { Label: "Attack target", Command: "attack ${target};;look" },
              { Label: "Wait for cue", Command: "$(wait-for \"ready\")" },
            ],
          },
        ],
      }],
    },
    DisplayProfiles: {
      DefaultID: DEFAULT_DISPLAY_PROFILE_ID,
      Profiles: [{
        ID: DEFAULT_DISPLAY_PROFILE_ID,
        Name: "Default",
        SidebarsVisible: true,
        Main: [
          { ComponentID: COMPONENT_IDS.all, SidebarHeight: 0, Collapsed: false },
          { ComponentID: CUSTOM_CHAT_ID, SidebarHeight: 0, Collapsed: false },
          { ComponentID: COMPONENT_IDS.metrics, SidebarHeight: 0, Collapsed: false },
          { ComponentID: COMPONENT_IDS.debug, SidebarHeight: 0, Collapsed: false },
        ],
        Sidebars: [
          {
            ID: "5afa1264-4ce7-41d0-b7ea-2050e0d34b07", Name: "Sidebar 1", Enabled: true, Side: "right", Width: 260,
            Components: [
              { ComponentID: COMPONENT_IDS.map, SidebarHeight: 160, Collapsed: false },
              { ComponentID: COMPONENT_IDS.exits, SidebarHeight: 220, Collapsed: false },
              { ComponentID: COMPONENT_IDS.vitals, SidebarHeight: 140, Collapsed: false },
              { ComponentID: COMPONENT_IDS.actions, SidebarHeight: 240, Collapsed: false },
              { ComponentID: COMPONENT_IDS.modes, SidebarHeight: 240, Collapsed: false },
              { ComponentID: COMPONENT_IDS.variables, SidebarHeight: 260, Collapsed: false },
            ],
          },
          { ID: "026686ea-d46b-4d20-94f1-e1c2485d7e7b", Name: "Sidebar 2", Enabled: false, Side: "right", Width: 260, Components: [] },
          { ID: "55148f61-39f1-4c16-be64-b9f3245e99bc", Name: "Sidebar 3", Enabled: false, Side: "right", Width: 260, Components: [] },
        ],
      }],
    },
    Scripts: ["/tmp/e2e-scripts"],
    UI: {
      DisplayMode: "sidebar",
      DefaultTab: "all",
      Scrollback: 5000,
      SidebarWidth: 40,
      GUISidebarWidth: 260,
      MinimapScale: 0.8,
      MinimapHeight: 12,
      GUIMinimapHeight: 160,
      CompassScale: 1,
      OutputFontSize: 14,
      // All three CRT effects OFF: the roll overlay animates forever and would
      // make layout assertions flaky.
      CRTScanlines: false,
      CRTRoll: false,
      CRTBloom: false,
      QuickCycleModes: ["disable"],
      ColorWords: false,
      EchoTyped: true,
      EchoScript: true,
      HideIPs: false,
      InputSpellcheck: false,
      KeepInputOnSend: false,
      MobileOutputFontSize: 14,
      MobileShowToolbar: true,
      MobileShowTabBar: true,
      MobileHideNavigationOnInput: false,
      MobileLowercaseFirstLetter: false,
      NumpadNavigation: "numlock",
    },
    Highlights: [],
    WoundHighlights: {
      Enabled: false,
      Cuts: "red",
      Bruises: "purple",
      Punctures: "blue",
      Burns: "orange",
    },
    SpeechColors: [],
    Kudos: { Favorites: [], Queue: [] },
    Ignorelist: { OOC: [], Think: [] },
    Notifications: {
      Desktop: {
        AllowScriptNotifications: false,
        Sound: false,
        HealthBelow: { Enabled: false, Threshold: 25 },
        FatigueBelow: { Enabled: false, Threshold: 10 },
        Patterns: [],
      },
    },
    Logging: { App: { Retain: false }, Session: { Enabled: false, Path: "" } },
    Updates: { Check: false },
    Onboarding: { WelcomeShown: true },
  };
}

export const baseInit: InitState = {
  version: VERSION,
  debug: false,
  accounts: [],
  credentialStore: {
    backend: "memory",
    available: true,
    canStore: true,
  },
  hasModes: true,
  modeNames: modeSpecs.map((m) => m.name),
  modeSpecs,
  colorwordStyles,
  activeCharacterProfileId: DEFAULT_CHARACTER_PROFILE_ID,
  config: makeConfig(),
};

export const withAccounts: InitState = { ...baseInit, accounts: ["hero"] };

// lines(n) → ["line 0001", …, "line n"] (zero-padded to 4) — distinct text so
// a burst test can locate the last one.
export function lines(n: number): string[] {
  return Array.from({ length: n }, (_, i) => `line ${String(i + 1).padStart(4, "0")}`);
}

// Sanitized shape of the 333-line inventory burst captured on 2026-09-20.
// The real output contained 111 identical tiara rows; generic placeholders
// preserve its size and repetition without committing session-log contents.
export function largeInventoryLines(): string[] {
  const result = ["You are wearing:"];
  result.push(...Array.from({ length: 110 }, () => "        a generic inventory item"));
  result.push(...Array.from({ length: 111 }, () => "        a gold tiara"));
  result.push(...Array.from({ length: 110 }, () => "        another generic inventory item"));
  result.push("End of inventory.");
  return result;
}

// One line with bold, italic, and colored segments.
export const styled: TextPayload = {
  text: "A bold word, an italic word, and a red word.",
  segments: [
    { text: "A " },
    { text: "bold", bold: true },
    { text: " word, an " },
    { text: "italic", italic: true },
    { text: " word, and a " },
    { text: "red", color: "#ff0000" },
    { text: " word." },
  ],
  timestamp: 0,
};
