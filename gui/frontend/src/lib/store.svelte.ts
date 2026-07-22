// Central reactive application state (Svelte 5 runes). A single store instance
// is shared across components. It ingests the WireEvent stream from the Go
// facade and maintains per-tab line buffers, status bars, metrics, graphics,
// and connection state.

import type {
  AppConfig,
  BarsPayload,
  ConnPayload,
  CustomTabConfig,
  Segment,
  StatusPayload,
  TextPayload,
  SuppressPayload,
  WireEvent,
} from "./types";
import { Kind } from "./types";

// Extra lines the scrollback buffer may hold above the configured cap before a
// front-trim runs, so the O(n) keyed-each reconciliation amortizes over a burst.
const TRIM_CHUNK = 256;

export type Screen = "loading" | "account" | "login" | "connecting" | "game";
export type TabKind = "all" | "custom" | "metrics" | "debug";

// Initial state for the Notes modal when opened by a /notes command. null (the
// menu path) means "open on the list view".
export type NotesInitial =
  | { view: "list" }
  | { view: "edit"; originalTitle: string; title: string; body: string };

export interface Line {
  id: number;
  segments: Segment[];
  isEcho: boolean;
  suppressed?: { placeholder: Segment[]; original: Segment[]; revealed: boolean };
}

interface CompiledRule {
  regex: RegExp;
  include: boolean;
  active: boolean;
}

export interface Tab {
  name: string;
  kind: TabKind;
  visible: boolean;
  rules: CompiledRule[];
  echoCommands: boolean;
  lines: Line[];
  unread: boolean;
}

export interface Toast {
  id: number;
  title: string;
  message: string;
}

// compileWildcard mirrors internal/ui/tabs.go compileWildcardPattern:
// case-insensitive substring match with * and ? wildcards.
function compileWildcard(pattern: string): RegExp {
  const escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, "\\$&") // QuoteMeta (minus * and ?)
    .replace(/\*/g, ".*")
    .replace(/\?/g, ".");
  try {
    return new RegExp(escaped, "i");
  } catch {
    return new RegExp(pattern.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "i");
  }
}

function matchesTab(text: string, rules: CompiledRule[]): boolean {
  let hasIncludes = false;
  let matched = false;
  for (const r of rules) {
    if (!r.active) continue;
    if (r.include) {
      hasIncludes = true;
      if (r.regex.test(text)) matched = true;
    } else if (r.regex.test(text)) {
      return false;
    }
  }
  if (!hasIncludes) return true;
  return matched;
}

function isExcludeOnly(rules: CompiledRule[]): boolean {
  for (const r of rules) if (r.active && r.include) return false;
  return true;
}

class AppStore {
  screen = $state<Screen>("loading");
  version = $state("dev");
  debug = $state(false);
  accounts = $state<string[]>([]);
  config = $state<AppConfig | null>(null);
  modeNames = $state<string[]>([]);
  hasModes = $state(false);

  tabs = $state<Tab[]>([]);
  activeTab = $state(0);

  // Status bars
  health = $state<number | null>(null);
  fatigue = $state<number | null>(null);
  encumbrance = $state<number | null>(null);
  satiation = $state<number | null>(null);
  lighting = $state<number | null>(null);
  lightingRaw = $state<number | null>(null);

  // Mode / state / metrics
  mode = $state("");
  status = $state<StatusPayload | null>(null);

  // Connection
  connState = $state<"connected" | "disconnected">("disconnected");
  connReason = $state("");
  // Browser-to-Praetor transport health is separate from the shared TEC
  // connection. Wails stays online; web mode updates this around snapshots and
  // reconnects so commands cannot be issued against stale state.
  transportReady = $state(true);
  transportState = $state<"connecting" | "connected" | "reconnecting">("connected");
  // Set when a disconnect was NOT user-initiated (a drop). Rendered as a banner
  // on the bootup screen; cleared on user logout and on the next connect.
  disconnectNotice = $state("");

  // Graphics
  minimap = $state<string>("");
  compass = $state<string>("");

  // UI chrome
  sidebarOpen = $state(true);
  // Collapsed state of the sidebar's accordion sections. Session-only (resets to
  // all-expanded on restart); held here rather than in Frame so it survives an
  // Alt+S sidebar unmount/remount.
  collapsed = $state({ map: false, exits: false, vitals: false });
  openModal = $state<string | null>(null);
  // True while the custom right-click context menu is open, so the game view's
  // Escape handler yields to it instead of opening the app menu.
  contextMenuOpen = $state(false);
  // Current Actions-tab set index. Ephemeral (not persisted): lifted out of
  // ActionsTab so the add-action modal can target the shown set and the
  // add-set modal can select the newly created set.
  actionSetIndex = $state(0);
  toasts = $state<Toast[]>([]);
  authError = $state("");
  loginUser = $state("");
  // Set to push text into the input line (e.g. kudos favorite prefill).
  inputPrefill = $state("");
  // Notes modal: initial view/note (set by a /notes command before opening),
  // whether the editor sub-view is active (so GameView routes Esc to it), and a
  // counter GameView bumps to ask the editor to step back to the list.
  notesInitial = $state<NotesInitial | null>(null);
  notesEditorActive = $state(false);
  notesBackRequest = $state(0);
  // Scrollback search (Ctrl+F). The open flag lives here so GameView's
  // capture-phase key routing, the OutputPane search bar, and Escape handling
  // all agree; the query itself is local to OutputPane.
  searchOpen = $state(false);
  // Bumped to ask the search bar to (re)focus and select its input.
  searchFocusRequest = $state(0);
  // Bumped to return keyboard focus to the game input (e.g. after closing the
  // search bar), since the sticky-focus logic only reacts to blur events.
  focusInputRequest = $state(0);
  // Reverse history search (Ctrl+R). GameView owns the capture-phase keydown,
  // so it bumps the request counter; InputLine performs the search and mirrors
  // its active state here so Escape routing can yield to it. Cancel is likewise
  // a counter so GameView can dismiss it from the window handler.
  histSearchRequest = $state(0);
  histSearchCancel = $state(0);
  histSearchActive = $state(false);
  // Global reveal of all suppressed lines (Alt+I), complementing per-line click.
  expandAllSuppressed = $state(false);
  // Where Esc goes from the currently-open modal: "menu" for submenus (with a
  // Back button), null to close entirely. Set by the active Modal on mount.
  modalEscapeTarget = $state<string | null>(null);

  private nextLineId = 1;
  private nextToastId = 1;

  get scrollback(): number {
    return this.config?.UI?.Scrollback ?? 5000;
  }

  // rebuildTabs (re)creates the tab list from config, preserving existing
  // line buffers by name where possible.
  rebuildTabs(customTabs: CustomTabConfig[] | null | undefined) {
    const prev = new Map(this.tabs.map((t) => [t.name, t.lines]));
    const tabs: Tab[] = [];
    tabs.push({
      name: "All",
      kind: "all",
      visible: true,
      rules: [],
      echoCommands: true,
      lines: prev.get("All") ?? [],
      unread: false,
    });
    for (const ct of customTabs ?? []) {
      tabs.push({
        name: ct.Name,
        kind: "custom",
        visible: ct.Visible,
        echoCommands: ct.EchoCommands,
        rules: (ct.Rules ?? []).map((r) => ({
          regex: compileWildcard(r.Pattern),
          include: r.Include,
          active: r.Active,
        })),
        lines: prev.get(ct.Name) ?? [],
        unread: false,
      });
    }
    tabs.push({ name: "Metrics", kind: "metrics", visible: true, rules: [], echoCommands: false, lines: [], unread: false });
    tabs.push({ name: "Debug", kind: "debug", visible: this.debug, rules: [], echoCommands: false, lines: [], unread: false });
    this.tabs = tabs;
    if (this.activeTab >= tabs.length) this.activeTab = 0;
  }

  // appendLine adds a line to a tab, caps scrollback, and marks the tab unread
  // when it is not the active tab (only for lines that "count" — non-blank
  // text, mirroring the TUI's unread rule).
  private appendLine(tab: Tab, line: Line, marksUnread = false) {
    tab.lines.push(line);
    const cap = this.scrollback;
    // Batch the front-trim: only trim once the buffer exceeds cap + TRIM_CHUNK,
    // then drop back to cap. Trimming one line per append (splice from the front)
    // triggers an O(n) reconciliation of the keyed {#each} on every append during
    // a burst; batching amortizes that cost over TRIM_CHUNK appends. The buffer
    // holds at most cap + TRIM_CHUNK lines (slightly more history, never less).
    if (cap > 0 && tab.lines.length > cap + TRIM_CHUNK) {
      tab.lines.splice(0, tab.lines.length - cap);
    }
    if (marksUnread && this.tabs[this.activeTab] !== tab) tab.unread = true;
  }

  private ingestText(p: TextPayload) {
    const line: Line = { id: this.nextLineId++, segments: p.segments ?? [], isEcho: !!p.isEcho };
    const mark = (p.text ?? "").trim() !== "";
    for (const tab of this.tabs) {
      if (tab.kind === "all") {
        this.appendLine(tab, line, mark);
      } else if (tab.kind === "custom") {
        if (!tab.visible) continue;
        if (line.isEcho && !tab.echoCommands && isExcludeOnly(tab.rules)) continue;
        if (matchesTab(p.text, tab.rules)) this.appendLine(tab, line, mark);
      }
    }
  }

  private ingestSuppressed(p: SuppressPayload) {
    const line: Line = {
      id: this.nextLineId++,
      segments: p.placeholderStyled ?? [],
      isEcho: false,
      suppressed: { placeholder: p.placeholderStyled ?? [], original: p.originalStyled ?? [], revealed: false },
    };
    const mark = (p.originalText ?? "").trim() !== "";
    for (const tab of this.tabs) {
      if (tab.kind === "all") {
        this.appendLine(tab, line, mark);
      } else if (tab.kind === "custom") {
        if (!tab.visible) continue;
        if (matchesTab(p.originalText, tab.rules)) this.appendLine(tab, line, mark);
      }
    }
  }

  private ingestDebug(channel: number, payload: string) {
    const tab = this.tabs.find((t) => t.kind === "debug");
    if (!tab) return;
    const line: Line = {
      id: this.nextLineId++,
      isEcho: false,
      segments: [
        { text: `[ch ${channel}] `, color: "#e8a838" },
        { text: payload, color: "#8a8a99" },
      ],
    };
    this.appendLine(tab, line, true);
  }

  // selectTab switches the active tab and clears its unread marker.
  selectTab(index: number) {
    if (index < 0 || index >= this.tabs.length) return;
    this.activeTab = index;
    this.tabs[index].unread = false;
  }

  private applyBars(b: BarsPayload) {
    if (b.hasHealth) this.health = b.health;
    if (b.hasFatigue) this.fatigue = b.fatigue;
    if (b.hasEncumbrance) this.encumbrance = b.encumbrance;
    if (b.hasSatiation) this.satiation = b.satiation;
    if (b.hasLighting) {
      this.lighting = b.lighting;
      this.lightingRaw = b.lightingRaw;
    }
  }

  // resetSession clears all game-session state (output, status, mode, graphics)
  // back to initial values, leaving config, accounts, and version intact.
  // Called when the connection ends (logout or drop) before returning to the
  // bootup screen.
  resetSession() {
    for (const tab of this.tabs) {
      tab.lines = [];
      tab.unread = false;
    }
    this.activeTab = 0;
    this.health = null;
    this.fatigue = null;
    this.encumbrance = null;
    this.satiation = null;
    this.lighting = null;
    this.lightingRaw = null;
    this.mode = "";
    this.status = null;
    this.minimap = "";
    this.compass = "";
    this.connReason = "";
    this.expandAllSuppressed = false;
    this.openModal = null;
    this.contextMenuOpen = false;
    this.actionSetIndex = 0;
    this.searchOpen = false;
    this.histSearchActive = false;
    this.notesInitial = null;
    this.notesEditorActive = false;
  }

  // installSnapshot replaces only shared game-session state. Config/account
  // metadata is delivered separately, while browser-local UI state such as
  // collapsed panels and input history remains owned by the browser.
  installSnapshot(events: WireEvent[]) {
    this.resetSession();
    this.connState = "disconnected";
    this.connReason = "";
    this.apply(events);
  }

  installConfig(config: AppConfig) {
    const oldTabs = this.config?.UI?.CustomTabs;
    this.config = config;
    if (config.UI?.CustomTabs !== oldTabs) {
      this.rebuildTabs(config.UI?.CustomTabs);
    }
  }

  private applyConn(c: ConnPayload) {
    this.connState = c.state;
    if (c.state === "connected") {
      // The authoritative signal that we're in-game.
      this.connReason = "";
      this.disconnectNotice = "";
      this.screen = "game";
      return;
    }
    // Disconnected: wipe session state and return to the bootup screen. A
    // non-empty reason means the socket dropped (not a user logout) — surface
    // it as a banner; an empty reason is a clean user-initiated logout.
    const reason = c.reason ?? "";
    this.resetSession();
    this.disconnectNotice = reason;
    this.screen = this.accounts.length > 0 ? "account" : "login";
  }

  addToast(title: string, message: string, durationMs = 6000) {
    const t: Toast = { id: this.nextToastId++, title, message };
    this.toasts.push(t);
    setTimeout(() => {
      this.toasts = this.toasts.filter((x) => x.id !== t.id);
    }, durationMs);
  }

  // addLocalLine appends a UI-generated informational line (dim) to the All tab.
  // Used by /notes list; this text is never sent to the game server.
  addLocalLine(text: string) {
    const all = this.tabs.find((t) => t.kind === "all");
    if (!all) return;
    const line: Line = {
      id: this.nextLineId++,
      segments: [{ text, color: "#8a8a99" }],
      isEcho: false,
    };
    this.appendLine(all, line, false);
  }

  // apply processes one batch of wire events in order.
  apply(batch: WireEvent[]) {
    for (const ev of batch) {
      // Once disconnected, ignore trailing in-game updates (text, status bars,
      // mode/metrics, graphics, debug) that raced the disconnect event — they
      // must not repopulate the session state resetSession() just cleared.
      // Connection, notification, and error events still apply.
      if (
        this.connState === "disconnected" &&
        ev.kind !== Kind.Conn &&
        ev.kind !== Kind.Notify &&
        ev.kind !== Kind.Error
      ) {
        continue;
      }
      switch (ev.kind) {
        case Kind.Text:
          if (ev.text) this.ingestText(ev.text);
          break;
        case Kind.Suppressed:
          if (ev.suppressed) this.ingestSuppressed(ev.suppressed);
          break;
        case Kind.Bars:
          if (ev.bars) this.applyBars(ev.bars);
          break;
        case Kind.Status:
          if (ev.status) {
            this.status = ev.status;
            this.mode = ev.status.mode;
          }
          break;
        case Kind.Conn:
          if (ev.conn) this.applyConn(ev.conn);
          break;
        case Kind.Notify:
          if (ev.notify) this.addToast(ev.notify.title, ev.notify.message);
          break;
        case Kind.Error:
          if (ev.error) this.addToast("Error: " + ev.error.context, ev.error.error);
          break;
        case Kind.Minimap:
          if (ev.image) this.minimap = ev.image.dataURI;
          break;
        case Kind.Compass:
          if (ev.image) this.compass = ev.image.dataURI;
          break;
        case Kind.Debug:
          if (ev.debug) this.ingestDebug(ev.debug.channel, ev.debug.payload);
          break;
        case Kind.OpenMenu:
          if (ev.openMenu) this.openModal = ev.openMenu;
          break;
        case Kind.Command:
          break; // command echo already arrives as a text event
      }
    }
  }
}

export const store = new AppStore();
