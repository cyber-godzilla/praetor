<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  // Options are "disable" plus every loaded mode. Seed from the current
  // snapshot, then refresh from the backend on open.
  let available = $state<string[]>(["disable", ...(store.modeNames ?? [])]);
  let selected = $state<Set<string>>(new Set(store.config?.UI?.QuickCycleModes ?? []));

  onMount(async () => {
    const names = await api.modeNames();
    if (names && names.length) store.modeNames = names;
    available = ["disable", ...(store.modeNames ?? [])];
  });

  function toggle(mode: string) {
    const s = new Set(selected);
    if (s.has(mode)) s.delete(mode);
    else s.add(mode);
    selected = s;
  }

  async function save() {
    // Keep the display order (disable first, then modes), then append any
    // selected entries not in the available list (stale config).
    const list = available.filter((m) => selected.has(m));
    for (const m of selected) if (!list.includes(m)) list.push(m);
    try {
      await api.setQuickCycleModes(list);
      store.config!.UI.QuickCycleModes = list;
      store.addToast("Quick-cycle modes", "Saved");
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="Quick-Cycle Modes (Alt+M)" back>
  <p class="hint dim">Check the modes to include in the Alt+M rotation.</p>
  <div class="list">
    {#each available as mode (mode)}
      <label class="row">
        <input type="checkbox" checked={selected.has(mode)} onchange={() => toggle(mode)} />
        <span class="name">{mode}</span>
      </label>
    {/each}
    {#if available.length <= 1}
      <div class="dim empty">No Lua modes loaded. Add script directories, then reload scripts.</div>
    {/if}
  </div>
  {#snippet footer()}
    <button onclick={() => (store.openModal = null)}>Cancel</button>
    <button class="primary" onclick={save}>Save</button>
  {/snippet}
</Modal>

<style>
  .hint {
    margin: 0 0 12px;
    font-size: 12px;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border-radius: 6px;
    font-size: 14px;
    cursor: pointer;
  }
  .row:hover {
    background: var(--bg-elevated);
  }
  .name {
    font-family: var(--mono);
  }
  .empty {
    padding: 14px;
    text-align: center;
  }
</style>
