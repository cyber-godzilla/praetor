<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { shouldRefocusInput, shouldRefocusFromClick, NON_REFOCUS_SELECTOR } from "../lib/focus";
  import { resolveModeName } from "../lib/modes";
  import { searchBackward, dropLastChar } from "../lib/histsearch";
  import { parseNotesCommand, formatNotesList } from "../lib/notescmd";
  import { insertsNewline, caretOnFirstLine, caretOnLastLine } from "../lib/multiline";
  import { isAllowedDuringPlay } from "../lib/playcmd";
  import type { PlayState } from "../lib/types";
  import CommandHint from "./CommandHint.svelte";
  import { matchCommands, tabComplete } from "../lib/commands";

  let value = $state("");
  let inputEl: HTMLTextAreaElement;
  let history: string[] = [];
  let histIdx = $state(-1); // -1 = current (not navigating)

  // Passive hint above the input. matchCommands returns [] for anything that is
  // not a slash command, so this is empty for ordinary game text.
  const hintMatches = $derived(
    matchCommands(value, { playing: store.playActive, modes: store.modeSpecs }),
  );

  // applyCompletion fills the input from a chosen hint row. completionFor only
  // ever returns an extension of what is typed, so this can never destroy the
  // line. The caret placement is deferred and explicit: focus() alone restores
  // the textarea's previous selection, and the DOM has not taken the new value
  // yet at this point.
  function applyCompletion(text: string) {
    value = text;
    histIdx = -1; // the line no longer reflects a history position
    queueMicrotask(() => {
      inputEl?.focus();
      inputEl?.setSelectionRange(text.length, text.length);
    });
  }

  // Mirror hint visibility for GameView's Tab routing. This must track the same
  // condition the markup below uses — the history search takes the slot, so the
  // hint is hidden while it runs. If the two disagree, Tab is swallowed with no
  // hint on screen.
  $effect(() => {
    store.hintActive =
      !store.histSearchActive && hintMatches.length > 0 && !store.openModal;
  });

  // GameView bumps this counter when Tab is pressed with the hint showing.
  let lastHintReq = 0;
  $effect(() => {
    const req = store.hintCompleteRequest;
    if (req === lastHintReq) return;
    lastHintReq = req;
    const next = tabComplete(value, hintMatches);
    if (next !== null) applyCompletion(next);
  });

  // Reverse history search (Ctrl+R), readline-style. Active state is mirrored
  // in store.histSearchActive so GameView's Escape routing can yield to it;
  // GameView also owns the Ctrl+R keydown (capture phase) and drives us via
  // the histSearchRequest counter.
  let rsQuery = $state("");
  let rsFailed = $state(false);
  let rsIndex = 0; // history index of the current match
  let rsSaved = ""; // input contents before the search began (restored on Esc)
  // Live performance status for the input-bar indicator. Polled only while a
  // performance is running: a step counter that visibly advances is what tells
  // the performer a long %wait or cue wait is a deliberate hold rather than a
  // wedged client — the whole reason this indicator exists.
  const IDLE_PLAY: PlayState = { active: false, paused: false, step: 0, total: 0 };
  let play = $state<PlayState>(IDLE_PLAY);

  // refreshPlay pulls the authoritative status once. Called by the poll and
  // immediately after a control action, so the indicator reflects a /pause or
  // /resume at once instead of up to a poll interval later.
  async function refreshPlay() {
    try {
      const st = await api.playStatus();
      play = st;
      if (!st.active) store.playActive = false;
    } catch {
      // A failed status call is not worth a toast mid-scene; the poll retries.
    }
  }

  // openPlayPicker is the /play flow, shared by the command and the indicator
  // button so the two can never drift apart.
  async function openPlayPicker() {
    if (store.connState !== "connected") {
      store.addToast("Play", "Not connected — nothing was played.");
      return;
    }
    try {
      const preview = await api.pickPlayFile();
      if (!preview.path) return; // picker cancelled
      store.playPreview = preview;
      store.openModal = "playscript";
    } catch (e) {
      store.addToast("Play", String(e));
    }
  }

  // The indicator doubles as the control: idle starts a script, running pauses,
  // paused resumes. /stop and Alt+X remain the ways to end a performance — this
  // button deliberately cannot, so a stray click can never destroy a scene.
  async function onPlayClick() {
    if (!play.active) {
      await openPlayPicker();
      return;
    }
    if (play.paused) await api.resumePlay();
    else await api.pausePlay();
    await refreshPlay();
  }

  $effect(() => {
    if (!store.playActive) {
      play = IDLE_PLAY;
      return;
    }
    let stopped = false;
    // The backend is the authority: a performance that ended on its own clears
    // store.playActive from its answer, which also tears this poll down.
    const tick = () => {
      if (!stopped) void refreshPlay();
    };
    tick();
    const id = setInterval(tick, 500);
    return () => {
      stopped = true;
      clearInterval(id);
    };
  });

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
    // Skip consecutive duplicates: with keep-input-on-send, re-sending the
    // same line is the normal workflow and must not flood ArrowUp history.
    if (line.trim() !== "" && line !== history[history.length - 1]) {
      history.push(line);
      if (history.length > 200) history.shift();
    }
    histIdx = -1;
    if (store.config?.UI?.KeepInputOnSend && line !== "") {
      // Keep the sent line and select it all: Enter re-sends it verbatim,
      // while any typing or Backspace replaces the whole thing. Assigning
      // `line` (not leaving `value` be) makes the kept text exactly what was
      // sent, even if the field changed during submit()'s awaits. Selection is
      // deferred: the bound textarea has not taken the value yet.
      value = line;
      queueMicrotask(() => inputEl?.select());
    } else {
      value = "";
    }
  }

  async function submit() {
    const line = value;
    const trimmed = line.trim();
    const lower = trimmed.toLowerCase();

    // Ask the backend rather than trusting a cached flag: a performance can end
    // on its own (script finished, send failed), and a stale "playing" flag
    // would lock the user out of their own input with no way back.
    const playing = await api.playActive();
    store.playActive = playing; // keep the UI hint in sync as a side effect
    if (playing && !isAllowedDuringPlay(line)) {
      store.addToast("Performance running", "Only /pause, /resume, /stop, /next (or Alt+X) are accepted.");
      pushHistory(line);
      return;
    }
    if (lower === "/pause") {
      pushHistory(line);
      if (!(await api.pausePlay())) store.addToast("Play", "Nothing is playing.");
      return;
    }
    if (lower === "/resume") {
      pushHistory(line);
      if (!(await api.resumePlay())) store.addToast("Play", "Nothing is paused.");
      return;
    }
    if (lower === "/stop") {
      pushHistory(line);
      store.playActive = false;
      if (!(await api.stopPlay())) store.addToast("Play", "Nothing is playing.");
      return;
    }
    if (lower === "/next") {
      pushHistory(line);
      await api.nextPlayStep();
      return;
    }
    if (lower === "/play") {
      pushHistory(line);
      await openPlayPicker();
      return;
    }

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
    if (lower === "/send") {
      pushHistory(line);
      if (store.connState !== "connected") {
        store.addToast("Send", "Not connected — nothing was sent.");
        return;
      }
      try {
        const preview = await api.pickSendFile();
        if (!preview.path) return; // picker cancelled
        store.sendPreview = preview;
        store.openModal = "sendfile";
      } catch (e) {
        store.addToast("Send", String(e));
      }
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
    // GameView (hold-to-walk, intentional). preventDefault is required here: on
    // a <textarea>, Enter's default action inserts a newline, so without it a
    // held Enter would fill the input with blank lines and silently turn the
    // next typed command into a multi-line block send.
    if (e.key === "Enter" && e.repeat) {
      e.preventDefault();
      return;
    }
    // Enter with any modifier inserts a line instead of sending. Return without
    // preventDefault so the textarea performs the insertion itself.
    if (insertsNewline(e)) return;
    if (store.histSearchActive && rsKeydown(e)) return;
    if (e.key === "Enter") {
      e.preventDefault();
      submit();
    } else if (e.key === "ArrowUp") {
      if (!caretOnFirstLine(value, inputEl.selectionStart)) return;
      if (history.length === 0) return;
      e.preventDefault();
      if (histIdx === -1) histIdx = history.length - 1;
      else if (histIdx > 0) histIdx--;
      value = history[histIdx] ?? "";
    } else if (e.key === "ArrowDown") {
      if (!caretOnLastLine(value, inputEl.selectionEnd)) return;
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

  // Grow the textarea with its content up to a cap, then scroll — Orchil caps at
  // 105px (resizeInput, orchil.js:1366). Height must be reset before measuring,
  // or scrollHeight only ever ratchets upward.
  const INPUT_MAX_PX = 105;
  function autosize() {
    if (!inputEl) return;
    inputEl.style.height = "auto";
    inputEl.style.height = Math.min(inputEl.scrollHeight, INPUT_MAX_PX) + "px";
  }
  $effect(() => {
    void value;
    queueMicrotask(autosize);
  });

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
  {:else if hintMatches.length > 0 && !store.openModal}
    <CommandHint matches={hintMatches} input={value} onchoose={applyCompletion} />
  {/if}
  <div class="inputbar">
    <span class="prompt">›</span>
    <textarea
      rows="1"
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
    ></textarea>
    <button
      class="play"
      class:active={play.active}
      class:paused={play.paused}
      title={!play.active
        ? "Play a script (/play)"
        : play.paused
          ? `Paused at step ${play.step} of ${play.total} — click to resume. /stop or Alt+X ends it.`
          : `Performing step ${play.step} of ${play.total} — click to pause. Only /pause, /resume, /stop, /next (or Alt+X) are accepted.`}
      onclick={onPlayClick}
      tabindex="-1"
    >
      {#if !play.active}
        ▶ play
      {:else}
        {play.paused ? "❙❙" : "▶"}
        {play.step}/{play.total}
      {/if}
    </button>
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
  textarea {
    flex: 1;
    font-size: 14px;
    background: var(--bg-input);
    border: 1px solid var(--border);
    resize: none;
    overflow-y: auto;
    font-family: inherit;
    line-height: 1.4;
  }
  /* Performance indicator. Electric blue rather than the orange accent: while
     this is showing, the input rejects everything but the four control
     commands, so it must not read as just another "a mode is active" state. */
  /* Idle it sits quiet like the mode button; performing it lights electric
     blue. Deliberately not the orange accent: while it is lit the input
     rejects everything but the four control commands, so it must read as a
     different class of state than "a mode is active". */
  .play {
    font-size: 12px;
    font-family: var(--mono);
    color: var(--fg-dim);
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 4px 10px;
    white-space: nowrap;
    user-select: none;
  }
  .play:hover {
    border-color: var(--play-blue);
    color: var(--play-blue);
  }
  .play.active {
    color: var(--play-blue);
    border-color: var(--play-blue-dim);
  }
  .play.active.paused {
    color: var(--play-blue-dim);
    border-color: var(--border);
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
