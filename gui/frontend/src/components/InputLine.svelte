<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { shouldRefocusInput, shouldRefocusFromClick, NON_REFOCUS_SELECTOR } from "../lib/focus";
  import { resolveModeName } from "../lib/modes";
  import { searchBackward, dropLastChar } from "../lib/histsearch";
  import { parseNotesCommand, formatNotesList } from "../lib/notescmd";

  let value = $state("");
  let inputEl: HTMLInputElement;
  let history: string[] = [];
  let histIdx = $state(-1); // -1 = current (not navigating)

  // Reverse history search (Ctrl+R), readline-style. Active state is mirrored
  // in store.histSearchActive so GameView's Escape routing can yield to it;
  // GameView also owns the Ctrl+R keydown (capture phase) and drives us via
  // the histSearchRequest counter.
  let rsQuery = $state("");
  let rsFailed = $state(false);
  let rsIndex = 0; // history index of the current match
  let rsSaved = ""; // input contents before the search began (restored on Esc)
  // True while a mouse button is held anywhere in the window — i.e. the user is
  // likely dragging out a text selection. Sticky-focus stands down until release
  // so it can't clear the selection mid-drag.
  let pointerDown = false;

  async function handleKudos(rest: string) {
    if (rest === "") {
      store.openModal = "kudos";
      return;
    }
    const m = rest.match(/^(\S+)(?:\s+(.*))?$/);
    if (!m) return;
    const name = m[1];
    const msg = (m[2] ?? "").trim();
    try {
      if (msg === "") {
        const added = await api.addKudosFavorite(name);
        store.addToast("Kudos", added ? `Added ${name} to favorites.` : `${name} is already a favorite.`);
      } else {
        await api.addKudosQueue(name, msg);
        store.addToast("Kudos", `Queued kudos for ${name}.`);
      }
      if (store.config) store.config.Kudos = await api.getKudos();
    } catch (e) {
      store.addToast("Kudos error", String(e));
    }
  }

  async function handleNotes(rest: string) {
    const cmd = parseNotesCommand(rest);
    switch (cmd.kind) {
      case "open-list":
        store.notesInitial = { view: "list" };
        store.openModal = "notes";
        break;
      case "new":
        store.notesInitial = { view: "edit", originalTitle: "", title: cmd.title, body: "" };
        store.openModal = "notes";
        break;
      case "open":
        try {
          const n = await api.getNote(cmd.title);
          store.notesInitial = { view: "edit", originalTitle: n.title, title: n.title, body: n.body };
          store.openModal = "notes";
        } catch {
          store.addToast("Notes", `No note titled "${cmd.title}".`);
        }
        break;
      case "delete":
        try {
          await api.deleteNote(cmd.title);
          store.addToast("Notes", `Deleted "${cmd.title}".`);
        } catch {
          store.addToast("Notes", `No note titled "${cmd.title}".`);
        }
        break;
      case "list": {
        try {
          const items = await api.listNotes();
          for (const l of formatNotesList(items)) store.addLocalLine(l);
        } catch (e) {
          store.addToast("Notes", String(e));
        }
        break;
      }
      case "usage":
        store.addToast("Notes", "Usage: /notes [add|open|delete|list] <title>");
        break;
    }
  }

  function pushHistory(line: string) {
    if (line.trim() !== "") {
      history.push(line);
      if (history.length > 200) history.shift();
    }
    histIdx = -1;
    value = "";
  }

  async function submit() {
    const line = value;
    const trimmed = line.trim();
    const lower = trimmed.toLowerCase();

    // Local commands handled by the UI (mirrors the TUI wrapper).
    if (lower === "/help") {
      store.openModal = "help";
      pushHistory(line);
      return;
    }
    if (lower === "/list") {
      // Open the mode selector (a columnar, clickable list) rather than a
      // comma-separated toast.
      store.openModal = "modeselect";
      pushHistory(line);
      return;
    }
    // /kudos family (open menu / add favorite / queue) — handled here because
    // the shared core does not interpret /kudos (it's a UI concern).
    if (lower === "/kudos" || lower.startsWith("/kudos ") || lower.startsWith("/kudos\t")) {
      await handleKudos(trimmed.slice("/kudos".length).trim());
      pushHistory(line);
      return;
    }
    if (lower === "/notes" || lower.startsWith("/notes ") || lower.startsWith("/notes\t")) {
      await handleNotes(trimmed.slice("/notes".length).trim());
      pushHistory(line);
      return;
    }
    if (lower.startsWith("/mode ") || lower.startsWith("/sm ")) {
      const parts = trimmed.split(/\s+/);
      const raw = parts[1];
      const args = parts.slice(2);
      const mode = raw ? resolveModeName(raw, store.modeNames) : raw;
      if (raw && mode === null) {
        store.addToast("Unknown mode", `"${raw}" — type /list to see available modes`);
        pushHistory(line);
        return;
      }
      try {
        await api.setMode(mode ?? "", args);
      } catch (e) {
        store.addToast("Mode error", String(e));
      }
      pushHistory(line);
      return;
    }

    // Everything else routes to the core (which interprets other /slash cmds).
    api.send(line);
    pushHistory(line);
  }

  // ---- Reverse history search --------------------------------------------

  function rsBegin() {
    store.histSearchActive = true;
    rsSaved = value;
    rsQuery = "";
    rsFailed = false;
    rsIndex = history.length; // first Ctrl+R step scans from the newest entry
  }

  // rsStep advances to the next-older match (repeat Ctrl+R).
  function rsStep() {
    const idx = searchBackward(history, rsQuery, rsIndex - 1);
    if (idx >= 0) {
      rsIndex = idx;
      value = history[idx];
      rsFailed = false;
    } else if (rsQuery !== "") {
      rsFailed = true;
    }
  }

  // rsSearchFresh re-runs the search from the newest entry (query changed).
  function rsSearchFresh() {
    const idx = searchBackward(history, rsQuery, history.length - 1);
    if (idx >= 0) {
      rsIndex = idx;
      value = history[idx];
      rsFailed = false;
    } else {
      rsFailed = rsQuery !== "";
      if (rsQuery === "") value = rsSaved;
    }
  }

  function rsAccept() {
    store.histSearchActive = false;
    histIdx = -1;
  }

  function rsCancel() {
    store.histSearchActive = false;
    value = rsSaved;
    histIdx = -1;
  }

  // GameView bumps these counters from its capture-phase key handling.
  let lastRsReq = 0;
  $effect(() => {
    const req = store.histSearchRequest;
    if (req === lastRsReq) return;
    lastRsReq = req;
    if (store.histSearchActive) rsStep();
    else rsBegin();
    inputEl?.focus();
  });
  let lastRsCancel = 0;
  $effect(() => {
    const req = store.histSearchCancel;
    if (req === lastRsCancel) return;
    lastRsCancel = req;
    if (store.histSearchActive) rsCancel();
  });

  // rsKeydown consumes keys while the history search is active. Returns true
  // when the event was fully handled; false lets normal handling run (after
  // the search has been accepted or ignored for that key).
  function rsKeydown(e: KeyboardEvent): boolean {
    if (e.key === "Enter") {
      e.preventDefault();
      rsAccept();
      submit();
      return true;
    }
    if (e.key === "Escape") {
      // Normally captured by GameView first; kept as a local fallback.
      e.preventDefault();
      rsCancel();
      return true;
    }
    if (e.key === "Backspace") {
      e.preventDefault();
      rsQuery = dropLastChar(rsQuery);
      rsSearchFresh();
      return true;
    }
    if (e.key === "ArrowUp" || e.key === "ArrowDown") {
      rsAccept(); // fall through: arrows resume normal history navigation
      return false;
    }
    if (e.key === "ArrowLeft" || e.key === "ArrowRight" || e.key === "Home" || e.key === "End" || e.key === "Tab") {
      rsAccept(); // accept the match and let the key do its usual thing
      return false;
    }
    if (e.key.length === 1 && !e.ctrlKey && !e.altKey && !e.metaKey) {
      e.preventDefault();
      rsQuery += e.key;
      rsSearchFresh();
      return true;
    }
    return false; // other chords (Ctrl+C, …) behave normally
  }

  function onKeydown(e: KeyboardEvent) {
    // IME composition: committing a candidate (Enter) or picking one (arrows)
    // must never submit or navigate history. keyCode 229 covers WebKit quirks
    // where isComposing is false on the final composition keystroke.
    if (e.isComposing || e.keyCode === 229) return;
    // Held Enter must not auto-fire repeated (blank/duplicate) submissions; the
    // first press still submits. Numpad movement repeat is handled separately in
    // GameView (hold-to-walk, intentional).
    if (e.key === "Enter" && e.repeat) return;
    if (store.histSearchActive && rsKeydown(e)) return;
    if (e.key === "Enter") {
      e.preventDefault();
      submit();
    } else if (e.key === "ArrowUp") {
      if (history.length === 0) return;
      e.preventDefault();
      if (histIdx === -1) histIdx = history.length - 1;
      else if (histIdx > 0) histIdx--;
      value = history[histIdx] ?? "";
    } else if (e.key === "ArrowDown") {
      if (histIdx === -1) return;
      e.preventDefault();
      if (histIdx < history.length - 1) {
        histIdx++;
        value = history[histIdx] ?? "";
      } else {
        histIdx = -1;
        value = "";
      }
    }
  }

  // Keep focus on the input whenever no modal is open.
  $effect(() => {
    if (!store.openModal && inputEl) inputEl.focus();
  });

  // Explicit refocus requests (e.g. after the Ctrl+F search bar closes — the
  // sticky-focus logic only reacts to blur events, so closing the bar would
  // otherwise strand focus on <body>).
  $effect(() => {
    void store.focusInputRequest;
    if (!store.openModal) inputEl?.focus();
  });

  // True when keyboard focus sits in some OTHER text field (the Ctrl+F search
  // box, a future inline editor) — sticky focus must stand down for it.
  function otherTextFieldActive(): boolean {
    const ae = document.activeElement as HTMLElement | null;
    return !!ae && ae !== inputEl && !!ae.closest?.(NON_REFOCUS_SELECTOR);
  }

  // Accept prefill pushes from other components (e.g. kudos favorites).
  $effect(() => {
    if (store.inputPrefill) {
      value = store.inputPrefill;
      store.inputPrefill = "";
      queueMicrotask(() => inputEl?.focus());
    }
  });

  // When the app window regains focus, put the cursor back in the input —
  // unless the browser restored focus to another text field (the search box).
  function onWindowFocus() {
    if (!store.openModal && !otherTextFieldActive()) inputEl?.focus();
  }

  // Webview window-focus events are unreliable, so treat a click anywhere in the
  // app as a signal to return the cursor to the input — unless it landed on a
  // text field or modal, or the user is selecting text (so copying still works).
  function refocusFromClick(e: MouseEvent) {
    const t = e.target as HTMLElement | null;
    const sel = window.getSelection();
    if (
      shouldRefocusFromClick({
        modalOpen: !!store.openModal,
        targetMatchesNonRefocus: !!t?.closest(NON_REFOCUS_SELECTOR),
        selectionCollapsed: !sel || sel.isCollapsed,
      })
    ) {
      inputEl?.focus();
    }
  }

  // Sticky focus: WebKitGTK moves focus on Tab/Shift+Tab (and clicks on
  // controls) at the GTK level, which DOM preventDefault/tabindex can't fully
  // stop — so whenever the input loses focus in the game view, snap it right
  // back. Modals are exempt so their fields keep focus. This is why Tab and
  // Shift+Tab cycle tabs (handled in GameView) rather than moving the focus
  // ring through the UI.
  function onBlur() {
    // Defer to the next frame: the blur fires before the browser settles which
    // element/selection the click landed on. Then only reclaim focus if the user
    // isn't selecting text (see shouldRefocusInput) — otherwise Ctrl+C / the
    // right-click Copy would have nothing to act on.
    requestAnimationFrame(() => {
      const sel = window.getSelection();
      if (
        shouldRefocusInput({
          modalOpen: !!store.openModal,
          pointerDown,
          selectionCollapsed: !sel || sel.isCollapsed,
          alreadyFocused: document.activeElement === inputEl,
          activeIsTextField: otherTextFieldActive(),
        })
      ) {
        inputEl?.focus();
      }
    });
  }
</script>

<svelte:window
  onfocus={onWindowFocus}
  onclick={refocusFromClick}
  onpointerdown={() => (pointerDown = true)}
  onpointerup={() => (pointerDown = false)}
  onpointercancel={() => (pointerDown = false)}
  onblur={() => (pointerDown = false)}
/>

<div class="inputwrap">
  {#if store.histSearchActive}
    <div class="rsearch" class:failed={rsFailed}>
      <span>(history search) “{rsQuery}”{rsFailed ? " — no match" : ""}</span>
      <span class="hint">Enter sends · Esc cancels · Ctrl+R older</span>
    </div>
  {/if}
  <div class="inputbar">
    <span class="prompt">›</span>
    <input
      type="text"
      bind:this={inputEl}
      bind:value
      onkeydown={onKeydown}
      oninput={() => {
        // Direct edits that bypass rsKeydown (paste, IME commit) implicitly
        // accept whatever is now in the field and end the search.
        if (store.histSearchActive) rsAccept();
      }}
      onblur={onBlur}
      spellcheck={store.config?.UI?.InputSpellcheck ?? true}
      autocomplete="off"
      placeholder={store.connState === "connected" ? "" : "(disconnected)"}
    />
    <button
      class="mode"
      class:active={!!store.mode && store.mode !== "disable"}
      title="Switch mode"
      onclick={() => (store.openModal = "modeselect")}
      tabindex="-1"
    >
      {store.mode && store.mode !== "disable" ? store.mode : "disable"}
    </button>
  </div>
</div>

<style>
  .inputwrap {
    position: relative;
  }
  .rsearch {
    position: absolute;
    bottom: 100%;
    left: 0;
    right: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 4px 12px;
    background: var(--bg-elevated);
    border-top: 1px solid var(--border);
    font-family: var(--mono);
    font-size: 12px;
    color: var(--fg);
  }
  .rsearch.failed {
    color: #cc6666;
  }
  .rsearch .hint {
    color: var(--fg-dim);
  }
  .inputbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: var(--bg-panel);
    border-top: 1px solid var(--border);
  }
  .prompt {
    color: var(--accent);
    font-family: var(--mono);
    font-size: 15px;
  }
  input {
    flex: 1;
    font-family: var(--mono);
    font-size: 14px;
    background: var(--bg-input);
    border: 1px solid var(--border);
  }
  .mode {
    font-size: 12px;
    font-family: var(--mono);
    color: var(--fg-dim);
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 4px 10px;
  }
  .mode:hover {
    border-color: var(--accent);
    color: var(--fg);
  }
  .mode.active {
    color: var(--accent);
    border-color: var(--accent-dim);
  }
</style>
