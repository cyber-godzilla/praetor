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

export type Screen = "loading" | "account" | "login" | "connecting" | "game";
export type TabKind = "all" | "custom" | "metrics" | "debug";

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
  displayState = $state<{ label: string; value: string }[]>([]);
  status = $state<StatusPayload | null>(null);

  // Connection
  connState = $state<"connected" | "disconnected" | "reconnecting">("disconnected");
  connReason = $state("");
  reconnectAttempt = $state(0);

  // Graphics
  minimap = $state<string>("");
  compass = $state<string>("");

  // UI chrome
  sidebarOpen = $state(true);
  openModal = $state<string | null>(null);
  toasts = $state<Toast[]>([]);
  authError = $state("");
  loginUser = $state("");
  // Set to push text into the input line (e.g. kudos favorite prefill).
  inputPrefill = $state("");
  // Global reveal of all suppressed lines (Alt+I), complementing per-line click.
  expandAllSuppressed = $state(false);

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
    if (cap > 0 && tab.lines.length > cap) {
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

  private applyConn(c: ConnPayload) {
    this.connState = c.state;
    this.connReason = c.reason ?? "";
    this.reconnectAttempt = c.attempt ?? 0;
    // The authoritative signal that we're in-game: the socket connected.
    if (c.state === "connected") this.screen = "game";
  }

  addToast(title: string, message: string) {
    const t: Toast = { id: this.nextToastId++, title, message };
    this.toasts.push(t);
    setTimeout(() => {
      this.toasts = this.toasts.filter((x) => x.id !== t.id);
    }, 6000);
  }

  // apply processes one batch of wire events in order.
  apply(batch: WireEvent[]) {
    for (const ev of batch) {
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
            this.displayState = ev.status.displayState ?? [];
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
