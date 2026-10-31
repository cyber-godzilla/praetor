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
  Variable: "variable",
  CharacterProfile: "characterProfile",
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
  speechSpeaker?: string;
  timestamp: number;
}

export interface SuppressPayload {
  channel: number;
  sourceName: string;
  placeholderText: string;
  placeholderStyled: Segment[];
  originalText: string;
  originalStyled: Segment[];
  speechSpeaker?: string;
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
  sound?: boolean;
  onlyWhenUnfocused?: boolean;
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
  variable?: VariablePayload;
  characterProfile?: CharacterProfilePayload;
  command?: string;
  openMenu?: string;
  image?: ImagePayload;
  debug?: DebugPayload;
}

export interface VariablePayload {
  name: string;
  value: string;
}

export interface CharacterProfilePayload {
  id: string;
  name: string;
}

// ---------------------------------------------------------------------------
// Config types (mirror internal/config; JSON keys are Go field names -> PascalCase)
// ---------------------------------------------------------------------------

export interface HighlightConfig {
  Pattern: string;
  Style: string; // red | gold | green | blue
  Active: boolean;
}

export interface WoundHighlightConfig {
  Enabled: boolean;
  Cuts: string;
  Bruises: string;
  Punctures: string;
  Burns: string;
}

export interface SpeechColorConfig {
  Character: string;
  Style: string;
  Active: boolean;
  IncludeThink: boolean;
  IncludeOOC: boolean;
}

export interface ColorwordStyle {
  name: string;
  label: string;
  family: string;
  kind: "solid" | "alternating" | "rainbow";
  colors: string[];
}

export interface TabRuleConfig {
  Pattern: string;
  Include: boolean;
  Active: boolean;
}

export interface CustomTabConfig {
  ID: string;
  Name: string;
  Visible: boolean;
  EchoCommands: boolean;
  Rules: TabRuleConfig[];
}

export interface DisplayComponentConfig {
  ComponentID: string;
  SidebarHeight: number;
  Collapsed: boolean;
}

export interface DisplaySidebarConfig {
  ID: string;
  Name: string;
  Enabled: boolean;
  Collapsed?: boolean;
  Side: "left" | "right";
  Width: number;
  Components: DisplayComponentConfig[];
}

export interface DisplayProfileConfig {
  ID: string;
  Name: string;
  CharacterProfileID?: string;
  SidebarsVisible: boolean;
  Main: DisplayComponentConfig[];
  Bench: DisplayComponentConfig[];
  Sidebars: DisplaySidebarConfig[];
}

export interface DisplayProfilesConfig {
  DefaultID: string;
  Profiles: DisplayProfileConfig[];
}

export interface ActionButton {
  Label: string;
  Command: string;
}

export interface ActionSet {
  ID: string;
  Name: string;
  Buttons: ActionButton[] | null;
}

export interface ThresholdConfig {
  Enabled: boolean;
  Threshold: number;
  When?: "always" | "unfocused";
  Sound?: boolean;
  Delivery?: "desktop" | "discord" | "both" | "neither";
}

export interface NotifyPatternConfig {
  Pattern: string;
  Title: string;
  Message: string;
  Enabled: boolean;
  When?: "always" | "unfocused";
  Sound?: boolean;
  Delivery?: "desktop" | "discord" | "both" | "neither";
}

export interface DesktopNotificationsConfig {
  AllowScriptNotifications: boolean;
  Sound: boolean;
  ScriptSound?: boolean;
  HealthBelow: ThresholdConfig;
  FatigueBelow: ThresholdConfig;
  Patterns: NotifyPatternConfig[] | null;
}

export interface DiscordNotificationsConfig {
  Enabled: boolean;
  WebhookURL: string;
  Health: boolean;
  Fatigue: boolean;
  Patterns: boolean;
  Scripts: boolean;
}

export interface NotificationsConfig {
  Desktop: DesktopNotificationsConfig;
  Discord: DiscordNotificationsConfig;
}

export interface KudosQueueEntry {
  Name: string;
  Message: string;
}

export interface KudosConfig {
  Favorites: string[] | null;
  Queue: KudosQueueEntry[] | null;
}

export interface CharacterProfileConfig {
  ID: string;
  Name: string;
  Account?: string;
  CharacterNumber?: number;
  VariablesMode?: "independent";
  Variables: Record<string, string> | null;
  CustomViews: CustomTabConfig[] | null;
  Highlights?: HighlightConfig[] | null;
  ActionSetsMode?: "inherit" | "customize";
  ActionSets: ActionSet[] | null;
  SelectedActionSetID: string;
  QuickCycleMode?: "inherit" | "customize";
  QuickCycleModes?: string[] | null;
  PriorityCommandsMode?: "inherit" | "customize";
  HighPriority?: string[] | null;
}

export interface CharacterProfileIdentity {
  ID: string;
  Name: string;
  Account: string;
  CharacterNumber: number;
}

export interface CharacterProfilesConfig {
  DefaultID: string;
  AutoSwitch: boolean;
  Profiles: CharacterProfileConfig[];
}

export interface UIConfig {
  DisplayMode: string;
  DefaultTab: string;
  Scrollback: number;
  SidebarWidth: number;
  GUISidebarWidth: number;
  MinimapScale: number;
  MinimapHeight: number;
  GUIMinimapHeight: number;
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
  InputSpellcheck: boolean;
  MobileOutputFontSize: number;
  MobileShowToolbar: boolean;
  MobileShowTabBar: boolean;
  MobileHideNavigationOnInput: boolean;
  MobileLowercaseFirstLetter: boolean;
  KeepInputOnSend: boolean;
  NumpadNavigation: string; // "numlock" | "always" | "off"
}

export interface AppConfig {
  Server: Record<string, unknown>;
  Commands: {
    SemicolonDelayMS: number;
    UnbusyDelayMS: number;
    HighPriority: string[] | null;
    [k: string]: unknown;
  };
  CharacterProfiles: CharacterProfilesConfig;
  DisplayProfiles: DisplayProfilesConfig;
  Scripts: string[] | null;
  DisabledScripts?: string[] | null;
  ManagedScripts?: ManagedScriptSourceConfig[] | null;
  UI: UIConfig;
  /** Legacy pre-profile field; new configurations store highlights per character. */
  Highlights?: HighlightConfig[] | null;
  WoundHighlights: WoundHighlightConfig;
  SpeechColors: SpeechColorConfig[] | null;
  Kudos: KudosConfig;
  Ignorelist: { OOC: string[] | null; Think: string[] | null };
  Notifications: NotificationsConfig;
  Logging: {
    App: { Retain: boolean };
    Session: { Enabled: boolean; Path: string };
    [k: string]: unknown;
  };
  Updates: { Check: boolean };
  Onboarding: { WelcomeShown: boolean };
}

export interface ManagedScriptSourceConfig {
  ID: string;
  Name: string;
  URL: string;
  Enabled: boolean;
  Trust: "restricted" | "trusted";
  CheckOnStartup: boolean;
  Tracking: "tag" | "branch";
  Ref: string;
  InstalledCommit: string;
  InstalledVersion?: string;
}

export interface ManagedScriptUpdateStatus {
  available: boolean;
  updated: boolean;
  version: string;
  commit: string;
  tracking: string;
  ref: string;
  error?: string;
}

export interface ManagedScriptSourceView {
  id: string;
  name: string;
  url: string;
  enabled: boolean;
  trust: string;
  checkOnStartup: boolean;
  tracking: string;
  ref: string;
  installedCommit: string;
  installedVersion: string;
  status: ManagedScriptUpdateStatus;
  canEnable: boolean;
}

export interface LocalScriptDirectoryView {
  path: string;
  enabled: boolean;
}

export interface ScriptManagerState {
  localDirectories: LocalScriptDirectoryView[];
  managed: ManagedScriptSourceView[];
  suggestedURL: string;
  suggestedInstalled: boolean;
}

// UpdateInfo mirrors internal/update.Info (json tags).
export interface UpdateInfo {
  available: boolean;
  current: string;
  latest: string;
  url: string;
}

// ModeSpec mirrors engine.ModeSpec: the metadata a Lua mode declares about
// itself. Unlike the static COMMANDS catalog this comes from whatever scripts
// the player has loaded, so /mode is the one command whose signature is
// resolved at runtime. An empty usage means the mode takes no arguments.
export interface ModeSpec {
  name: string;
  usage: string;
  desc: string;
  chains: boolean;
  // Keep this mode out of the command hint. A display hint only — the mode
  // stays loaded, /mode <name> still runs it, and the mode picker still lists
  // it, so hiding declutters typing without hiding the mode itself.
  hidden?: boolean;
}

export interface InitState {
  version: string;
  debug: boolean;
  accounts: string[] | null;
  credentialStore: CredentialStoreStatus;
  hasModes: boolean;
  modeNames: string[] | null;
  modeSpecs: ModeSpec[] | null;
  colorwordStyles: ColorwordStyle[] | null;
  activeCharacterProfileId: string;
  config: AppConfig;
}

export interface CredentialStoreStatus {
  backend: string;
  available: boolean;
  canStore: boolean;
  message?: string;
}

export interface AccountState {
  accounts: string[];
  credentialStore: CredentialStoreStatus;
}

export interface ConnectResult {
  connected: boolean;
  credentialSaveRequested: boolean;
  credentialsSaved: boolean;
  warning?: string;
  accountState?: AccountState;
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

export interface TrainingCostRow {
  slot: number;
  basic: number;
  easy: number;
  average: number;
  difficult: number;
  impossible: number;
}

export interface NoteSummary {
  title: string;
  preview: string;
}

export interface Note {
  title: string;
  body: string;
}

export interface SendPreview {
  path: string;
  name: string;
  lines: number;
  batches: number;
}

export interface PlayError {
  line: number;
  message: string;
}

// Live status of a performance, polled while one is running so the input-bar
// indicator can show progress. Step is 1-based.
export interface PlayState {
  active: boolean;
  paused: boolean;
  step: number;
  total: number;
}

export interface InputChainStatus {
  active: boolean;
  chains: number;
  step: number;
  total: number;
  state: string;
  detail?: string;
  durationMs?: number;
  remainingMs?: number;
  attempts?: number;
  maxAttempts?: number;
}

export interface PlayPreview {
  path: string;
  name: string;
  steps: number;
  fixedMs: number;
  hasCues: boolean;
  errors: PlayError[] | null;
}
