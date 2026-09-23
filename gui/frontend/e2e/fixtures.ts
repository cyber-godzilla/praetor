import type { AppConfig, InitState, ModeSpec, TextPayload } from "../src/lib/types";

// Everything the fake backend hands the frontend. Keep this the ONE place
// canned state lives so a config-shape change is a one-file fix.

export const VERSION = "v0.0.0-e2e";

export const modeSpecs: ModeSpec[] = [
  { name: "disable", usage: "", desc: "Turn automation off", chains: false },
  { name: "hunt", usage: "[target]", desc: "Hunt the given target", chains: false },
  { name: "hunt_wolves", usage: "", desc: "Hunt wolves specifically", chains: true },
];

export function makeConfig(): AppConfig {
  return {
    Server: {},
    Commands: { HighPriority: [], Variables: {} },
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
      NumpadNavigation: "numlock",
      CustomTabs: [
        {
          Name: "Chat",
          Visible: true,
          EchoCommands: false,
          Rules: [{ Pattern: "says,", Include: true, Active: true }],
        },
      ],
      ActionSets: [
        {
          Name: "Combat",
          Buttons: [{ Label: "Attack target", Command: "attack ${target};;look" }],
        },
      ],
    },
    Highlights: [],
    Kudos: { Favorites: [], Queue: [] },
    Ignorelist: { OOC: [], Think: [] },
    Notifications: {
      Desktop: {
        Sound: false,
        HealthBelow: { Enabled: false, Threshold: 25 },
        FatigueBelow: { Enabled: false, Threshold: 10 },
        Patterns: [],
      },
    },
    Logging: { Session: { Enabled: false, Path: "" } },
    Updates: { Check: false },
    Onboarding: { WelcomeShown: true },
  };
}

export const baseInit: InitState = {
  version: VERSION,
  debug: false,
  accounts: [],
  hasModes: true,
  modeNames: modeSpecs.map((m) => m.name),
  modeSpecs,
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
