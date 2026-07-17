// Wire types mirroring internal/gui/payloads.go. Field names match the Go
// json tags exactly.

export interface Segment {
  text: string;
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  color?: string;
  isHR?: boolean;
  bg?: string; // frontend-only: highlight background
}

export const Kind = {
  Text: "text",
  Suppressed: "suppressed",
  Status: "status",
  Bars: "bars",
  Conn: "conn",
  Notify: "notify",
  Error: "error",
  Command: "command",
  OpenMenu: "openMenu",
  Minimap: "minimap",
  Compass: "compass",
  Debug: "debug",
} as const;

export interface TextPayload {
  text: string;
  segments: Segment[];
  raw?: string;
  isEcho?: boolean;
  timestamp: number;
}

export interface SuppressPayload {
  channel: number;
  sourceName: string;
  placeholderText: string;
  placeholderStyled: Segment[];
  originalText: string;
  originalStyled: Segment[];
  timestamp: number;
}

export interface MetricEntry {
  label: string;
  value: number;
}

export interface MetricSession {
  mode: string;
  start: number;
  end: number;
  durationMs: number;
  entries: MetricEntry[];
}

export interface StatusPayload {
  mode: string;
  current?: MetricSession;
  history?: MetricSession[];
}

export interface BarsPayload {
  hasHealth: boolean;
  health: number;
  hasFatigue: boolean;
  fatigue: number;
  hasEncumbrance: boolean;
  encumbrance: number;
  hasSatiation: boolean;
  satiation: number;
  hasLighting: boolean;
  lighting: number;
  lightingRaw: number;
}

export interface ConnPayload {
  state: "connected" | "disconnected";
  reason?: string;
}

export interface NotifyPayload {
  title: string;
  message: string;
}

export interface ErrorPayload {
  context: string;
  error: string;
}

export interface ImagePayload {
  dataURI: string;
  width: number;
  height: number;
}

export interface DebugPayload {
  channel: number;
  payload: string;
}

export interface WireEvent {
  kind: string;
  text?: TextPayload;
  suppressed?: SuppressPayload;
  status?: StatusPayload;
  bars?: BarsPayload;
  conn?: ConnPayload;
  notify?: NotifyPayload;
  error?: ErrorPayload;
  command?: string;
  openMenu?: string;
  image?: ImagePayload;
  debug?: DebugPayload;
}

// ---------------------------------------------------------------------------
// Config types (mirror internal/config; JSON keys are Go field names -> PascalCase)
// ---------------------------------------------------------------------------

export interface HighlightConfig {
  Pattern: string;
  Style: string; // red | gold | green | blue
  Active: boolean;
}

export interface TabRuleConfig {
  Pattern: string;
  Include: boolean;
  Active: boolean;
}

export interface CustomTabConfig {
  Name: string;
  Visible: boolean;
  EchoCommands: boolean;
  Rules: TabRuleConfig[];
}

export interface ActionButton {
  Label: string;
  Command: string;
}

export interface ActionSet {
  Name: string;
  Buttons: ActionButton[] | null;
}

export interface ThresholdConfig {
  Enabled: boolean;
  Threshold: number;
}

export interface NotifyPatternConfig {
  Pattern: string;
  Title: string;
  Message: string;
  Enabled: boolean;
}

export interface DesktopNotificationsConfig {
  HealthBelow: ThresholdConfig;
  FatigueBelow: ThresholdConfig;
  Patterns: NotifyPatternConfig[] | null;
}

export interface KudosQueueEntry {
  Name: string;
  Message: string;
}

export interface KudosConfig {
  Favorites: string[] | null;
  Queue: KudosQueueEntry[] | null;
}

export interface UIConfig {
  DisplayMode: string;
  DefaultTab: string;
  Scrollback: number;
  SidebarWidth: number;
  MinimapScale: number;
  MinimapHeight: number;
  CompassScale: number;
  OutputFontSize: number;
  CRTScanlines: boolean;
  CRTRoll: boolean;
  CRTBloom: boolean;
  QuickCycleModes: string[] | null;
  ColorWords: boolean;
  EchoTyped: boolean;
  EchoScript: boolean;
  HideIPs: boolean;
  NumpadNavigation: string; // "numlock" | "always" | "off"
  CustomTabs: CustomTabConfig[] | null;
  ActionSets: ActionSet[] | null;
}

export interface AppConfig {
  Server: Record<string, unknown>;
  Commands: { HighPriority: string[] | null; [k: string]: unknown };
  Scripts: string[] | null;
  UI: UIConfig;
  Highlights: HighlightConfig[] | null;
  Kudos: KudosConfig;
  Ignorelist: { OOC: string[] | null; Think: string[] | null };
  Notifications: { Desktop: DesktopNotificationsConfig };
  Logging: {
    Session: { Enabled: boolean; Path: string };
    [k: string]: unknown;
  };
}

export interface InitState {
  version: string;
  debug: boolean;
  accounts: string[] | null;
  hasModes: boolean;
  modeNames: string[] | null;
  config: AppConfig;
}

export interface PersistentKeyInfo {
  key: string;
  valueSummary: string;
}

export interface WikiBookmark {
  Key: string;
  Slug: string;
}

export interface WikiSection {
  Name: string;
  Bookmarks: WikiBookmark[];
}

export interface RBCell {
  posture: number;
  difficulty: number;
  bonus: number;
}

export interface RBResult {
  mode: number;
  basics: number;
  subskill: number;
  basicsRB: number;
  subskillRB: number;
  cells: RBCell[];
}
