<script lang="ts">
  import { onDestroy, tick } from "svelte";
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { shouldRefocusInput, shouldRefocusFromClick, NON_REFOCUS_SELECTOR } from "../lib/focus";
  import {
    lowercaseFirstCommandLetter,
    MOBILE_COMMAND_QUERY,
    MOBILE_LAYOUT_QUERY,
    outerPageHasVerticalDrift,
  } from "../lib/mobile";
  import { resolveModeName } from "../lib/modes";
  import { searchBackward, dropLastChar } from "../lib/histsearch";
  import { parseNotesCommand, formatNotesList } from "../lib/notescmd";

  let value = $state("");
  let inputEl: HTMLInputElement;
  let history: string[] = [];
  let histIdx = $state(-1); // -1 = current (not navigating)
  let submitting = $state(false);

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
  let mobileToolPointerDown = false;
  let mobileToolReleaseTimer: ReturnType<typeof setTimeout> | undefined;
  const MOBILE_KEYBOARD_DEBOUNCE_MS = 140;
  const MOBILE_KEYBOARD_FALLBACK_MS = 360;
  const MOBILE_KEYBOARD_OBSERVE_MS = 1200;
  let keyboardDebounceTimer: ReturnType<typeof setTimeout> | undefined;
  let keyboardFallbackTimer: ReturnType<typeof setTimeout> | undefined;
  let keyboardObserveTimer: ReturnType<typeof setTimeout> | undefined;
  let observedVisualViewport: VisualViewport | null = null;

  // Desktop keeps a sticky command cursor. Touch-sized/coarse-pointer clients
  // only focus after an explicit tap so the software keyboard does not reopen
  // after every command or modal interaction.
  function stickyFocusEnabled(): boolean {
    return !window.matchMedia(MOBILE_COMMAND_QUERY).matches;
  }

  function mobileWebInput(): boolean {
    return api.inWeb() && window.matchMedia(MOBILE_COMMAND_QUERY).matches;
  }

  function mobileWebLayout(): boolean {
    return api.inWeb() && window.matchMedia(MOBILE_LAYOUT_QUERY).matches;
  }

  // Opening a software keyboard can pan the browser's outer viewport after the
  // mobile dock has disappeared and App has resized to visualViewport.height.
  // Correct that browser-owned scroll only after the keyboard/layout settles;
  // the OutputPane's own scrollTop is intentionally never read or changed here.
  function restoreOuterPageTopIfShifted() {
    if (!mobileWebLayout() || document.activeElement !== inputEl) return;

    const scrollingElement = document.scrollingElement;
    if (
      !outerPageHasVerticalDrift({
        windowY: window.scrollY,
        scrollingElementTop: scrollingElement?.scrollTop ?? 0,
        documentElementTop: document.documentElement.scrollTop,
        bodyTop: document.body.scrollTop,
        visualViewportOffsetTop: window.visualViewport?.offsetTop ?? 0,
      })
    ) {
      return;
    }

    // Some mobile engines expose the keyboard pan through window.scrollY,
    // while others retain it on the root/body scrolling element. Clear each
    // representation so the session output remains aligned with the screen top.
    window.scrollTo(0, 0);
    if (scrollingElement) scrollingElement.scrollTop = 0;
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  }

  function scheduleKeyboardViewportCorrection() {
    if (keyboardDebounceTimer !== undefined) clearTimeout(keyboardDebounceTimer);
    keyboardDebounceTimer = setTimeout(() => {
      keyboardDebounceTimer = undefined;
      restoreOuterPageTopIfShifted();
    }, MOBILE_KEYBOARD_DEBOUNCE_MS);
  }

  function stopKeyboardViewportCorrection() {
    if (observedVisualViewport) {
      observedVisualViewport.removeEventListener("resize", scheduleKeyboardViewportCorrection);
      observedVisualViewport.removeEventListener("scroll", scheduleKeyboardViewportCorrection);
      observedVisualViewport = null;
    }
    if (keyboardDebounceTimer !== undefined) clearTimeout(keyboardDebounceTimer);
    if (keyboardFallbackTimer !== undefined) clearTimeout(keyboardFallbackTimer);
    if (keyboardObserveTimer !== undefined) clearTimeout(keyboardObserveTimer);
    keyboardDebounceTimer = undefined;
    keyboardFallbackTimer = undefined;
    keyboardObserveTimer = undefined;
  }

  function startKeyboardViewportCorrection() {
    stopKeyboardViewportCorrection();
    if (!mobileWebLayout()) return;

    observedVisualViewport = window.visualViewport;
    observedVisualViewport?.addEventListener("resize", scheduleKeyboardViewportCorrection);
    observedVisualViewport?.addEventListener("scroll", scheduleKeyboardViewportCorrection);

    // A delayed fallback covers browsers that animate the keyboard without
    // dispatching VisualViewport events. The final check closes this bounded
    // observation window after slower keyboard animations have completed.
    keyboardFallbackTimer = setTimeout(() => {
      keyboardFallbackTimer = undefined;
      restoreOuterPageTopIfShifted();
    }, MOBILE_KEYBOARD_FALLBACK_MS);
    keyboardObserveTimer = setTimeout(() => {
      restoreOuterPageTopIfShifted();
      stopKeyboardViewportCorrection();
    }, MOBILE_KEYBOARD_OBSERVE_MS);

    // Focusing also removes the optional map/compass dock. Check again after
    // Svelte has committed that shorter layout and the browser has painted it;
    // this catches page drift caused by the dock collapse independently of the
    // later keyboard animation.
    void tick().then(() => {
      requestAnimationFrame(() => {
        if (document.activeElement === inputEl && mobileWebLayout()) {
          scheduleKeyboardViewportCorrection();
        }
      });
    });
  }

  function normalizeMobileCommand(input: string): string {
    if (!mobileWebInput() || !store.config?.UI?.MobileLowercaseFirstLetter) return input;
    return lowercaseFirstCommandLetter(input);
  }

  function onInput(e: Event) {
    // Paste and IME commits accept the current reverse-history match before
    // applying mobile capitalization normalization.
    if (store.histSearchActive) rsAccept();
    const target = e.currentTarget as HTMLInputElement;
    const original = target.value;
    const normalized = normalizeMobileCommand(original);
    value = normalized;
    if (normalized === original) return;

    // Lowercasing can change UTF-16 length for a small number of Unicode
    // characters. Keep the native selection aligned instead of jumping to the
    // end after correcting the software keyboard's capitalization.
    const delta = normalized.length - original.length;
    const start = target.selectionStart;
    const end = target.selectionEnd;
    let changedAt = 0;
    while (changedAt < original.length && original[changedAt] === normalized[changedAt]) {
      changedAt++;
    }
    const adjust = (position: number) =>
      position > changedAt ? position + delta : position;
    target.value = normalized;
    if (start !== null && end !== null) {
      target.setSelectionRange(Math.max(0, adjust(start)), Math.max(0, adjust(end)));
    }
  }

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

  async function submitOnce() {
    const line = normalizeMobileCommand(value);
    if (line !== value) value = line;
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
    await api.send(line);
    pushHistory(line);
  }

  async function submit() {
    if (submitting || !store.transportReady) return;
    submitting = true;
    try {
      await submitOnce();
    } catch (error) {
      store.addToast("Command failed", error instanceof Error ? error.message : String(error));
    } finally {
      submitting = false;
      // A disabled input is blurred by browsers while the request is in
      // flight. Wait for Svelte to enable it again before restoring desktop
      // focus; mobile preserves its tap-to-focus policy.
      await tick();
      if (!store.openModal && store.transportReady && stickyFocusEnabled()) {
        inputEl?.focus();
      }
    }
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
    if (!store.openModal && inputEl && stickyFocusEnabled()) inputEl.focus();
  });

  // Explicit refocus requests (e.g. after the Ctrl+F search bar closes — the
  // sticky-focus logic only reacts to blur events, so closing the bar would
  // otherwise strand focus on <body>).
  $effect(() => {
    void store.focusInputRequest;
    if (!store.openModal && stickyFocusEnabled()) inputEl?.focus();
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
      if (stickyFocusEnabled()) queueMicrotask(() => inputEl?.focus());
    }
  });

  // When the app window regains focus, put the cursor back in the input —
  // unless the browser restored focus to another text field (the search box).
  function onWindowFocus() {
    if (!store.openModal && !otherTextFieldActive() && stickyFocusEnabled()) inputEl?.focus();
  }

  function onPointerDown(e: PointerEvent) {
    pointerDown = true;
    const target = e.target as HTMLElement | null;
    mobileToolPointerDown = !!target?.closest("[data-mobile-tools]");
  }

  function onPointerUp() {
    pointerDown = false;
    if (!mobileToolPointerDown) return;
    mobileToolPointerDown = false;
    // `click` follows `pointerup`; defer cleanup so the tool can open its modal
    // before restoring navigation changes the dock's geometry.
    mobileToolReleaseTimer = setTimeout(() => {
      if (!store.openModal && document.activeElement !== inputEl) {
        store.mobileCommandInputActive = false;
      }
    }, 0);
  }

  function onPointerCancel() {
    pointerDown = false;
    mobileToolPointerDown = false;
    if (document.activeElement !== inputEl) store.mobileCommandInputActive = false;
  }

  function onWindowBlur() {
    pointerDown = false;
    mobileToolPointerDown = false;
    stopKeyboardViewportCorrection();
    store.mobileCommandInputActive = false;
  }

  // Webview window-focus events are unreliable, so treat a click anywhere in the
  // app as a signal to return the cursor to the input — unless it landed on a
  // text field or modal, or the user is selecting text (so copying still works).
  function refocusFromClick(e: MouseEvent) {
    if (!stickyFocusEnabled()) return;
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
  function onFocus() {
    store.mobileCommandInputActive = mobileWebLayout();
    startKeyboardViewportCorrection();
  }

  function onBlur(e: FocusEvent) {
    stopKeyboardViewportCorrection();
    const next = e.relatedTarget as HTMLElement | null;
    // Keep the compact dock stable until a focusable mobile tool has received
    // its click. The tool handler clears this flag after dispatch, preventing
    // the hidden map from reappearing and moving the target mid-tap.
    if (!mobileToolPointerDown && !next?.closest("[data-mobile-tools]")) {
      store.mobileCommandInputActive = false;
    }
    // Defer to the next frame: the blur fires before the browser settles which
    // element/selection the click landed on. Then only reclaim focus if the user
    // isn't selecting text (see shouldRefocusInput) — otherwise Ctrl+C / the
    // right-click Copy would have nothing to act on.
    requestAnimationFrame(() => {
      if (!stickyFocusEnabled()) return;
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

  onDestroy(() => {
    if (mobileToolReleaseTimer !== undefined) clearTimeout(mobileToolReleaseTimer);
    stopKeyboardViewportCorrection();
    store.mobileCommandInputActive = false;
  });
</script>

<svelte:window
  onfocus={onWindowFocus}
  onclick={refocusFromClick}
  onpointerdown={onPointerDown}
  onpointerup={onPointerUp}
  onpointercancel={onPointerCancel}
  onblur={onWindowBlur}
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
      oninput={onInput}
      onfocus={onFocus}
      onblur={onBlur}
      spellcheck={store.config?.UI?.InputSpellcheck ?? true}
      autocomplete="off"
      autocapitalize={api.inWeb() && (store.config?.UI?.MobileLowercaseFirstLetter ?? false) ? "none" : "sentences"}
      disabled={!store.transportReady || submitting}
      placeholder={store.connState === "connected" ? "" : "(disconnected)"}
    />
    <button class="send" onclick={submit} aria-label="Send command" disabled={!store.transportReady || submitting}>Send</button>
    <button
      class="mode"
      class:active={!!store.mode && store.mode !== "disable"}
      title="Switch mode"
      onclick={() => (store.openModal = "modeselect")}
      disabled={!store.transportReady}
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
  .send {
    display: none;
  }

  @media (max-width: 899px), (pointer: coarse) {
    .inputbar {
      gap: 6px;
      padding: 7px 8px;
    }
    .prompt {
      display: none;
    }
    input {
      min-width: 0;
      min-height: 44px;
      font-size: 16px;
    }
    .send {
      display: block;
      min-width: 58px;
      min-height: 44px;
      border-color: var(--accent);
      color: var(--accent);
    }
    .mode {
      min-height: 44px;
      max-width: 92px;
      overflow: hidden;
      padding-inline: 8px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }
</style>
