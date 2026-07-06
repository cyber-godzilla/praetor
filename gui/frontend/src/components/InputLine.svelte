<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let value = $state("");
  let inputEl: HTMLInputElement;
  let history: string[] = [];
  let histIdx = $state(-1); // -1 = current (not navigating)

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
      const modes = store.modeNames ?? [];
      store.addToast("Available modes", modes.length ? modes.join(", ") : "no modes loaded");
      pushHistory(line);
      return;
    }
    if (lower.startsWith("/mode ") || lower.startsWith("/sm ")) {
      const parts = trimmed.split(/\s+/);
      const mode = parts[1];
      const args = parts.slice(2);
      if (mode && mode !== "disable" && !(store.modeNames ?? []).includes(mode)) {
        store.addToast("Unknown mode", `"${mode}" — type /list to see available modes`);
        pushHistory(line);
        return;
      }
      try {
        await api.setMode(mode, args);
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
</script>

<div class="inputbar">
  <span class="prompt">›</span>
  <input
    type="text"
    bind:this={inputEl}
    bind:value
    onkeydown={onKeydown}
    spellcheck="false"
    autocomplete="off"
    placeholder={store.connState === "connected" ? "" : "(disconnected)"}
  />
  {#if store.mode && store.mode !== "disable"}
    <span class="mode" title="Active mode">{store.mode}</span>
  {/if}
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
    color: var(--accent);
    background: var(--bg-elevated);
    border: 1px solid var(--accent-dim);
    border-radius: 4px;
    padding: 3px 8px;
  }
</style>
