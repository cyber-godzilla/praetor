<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { shouldRefocusInput, shouldRefocusFromClick, NON_REFOCUS_SELECTOR } from "../lib/focus";
  import { resolveModeName } from "../lib/modes";

  let value = $state("");
  let inputEl: HTMLInputElement;
  let history: string[] = [];
  let histIdx = $state(-1); // -1 = current (not navigating)
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

  function onKeydown(e: KeyboardEvent) {
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

  // Accept prefill pushes from other components (e.g. kudos favorites).
  $effect(() => {
    if (store.inputPrefill) {
      value = store.inputPrefill;
      store.inputPrefill = "";
      queueMicrotask(() => inputEl?.focus());
    }
  });

  // When the app window regains focus, put the cursor back in the input.
  function onWindowFocus() {
    if (!store.openModal) inputEl?.focus();
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

<div class="inputbar">
  <span class="prompt">›</span>
  <input
    type="text"
    bind:this={inputEl}
    bind:value
    onkeydown={onKeydown}
    onblur={onBlur}
    spellcheck="false"
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

<style>
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
