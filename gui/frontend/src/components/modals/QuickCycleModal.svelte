<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  // Options are "disable" plus every loaded mode. Seed from the current
  // snapshot, then refresh from the backend on open.
  // Dedupe so a script-defined "disable" mode can't collide with the built-in
  // "disable" entry and crash the keyed {#each} (each_key_duplicate).
  const modeList = () => [...new Set(["disable", ...(store.modeNames ?? [])])];
  let available = $state<string[]>(modeList());
  const profile = store.characterProfile();
  const isDefault = profile?.ID === store.config?.CharacterProfiles.DefaultID;
  let inheriting = $state(!isDefault && profile?.QuickCycleMode === "inherit");
  let selected = $state<Set<string>>(new Set(store.resolvedQuickCycleModes()));

  onMount(async () => {
    const names = await api.modeNames();
    if (names && names.length) store.modeNames = names;
    available = modeList();
  });

  function toggle(mode: string) {
    const s = new Set(selected);
    if (s.has(mode)) s.delete(mode);
    else s.add(mode);
    selected = s;
  }

  function customize() {
    inheriting = false;
    selected = new Set(store.resolvedQuickCycleModes());
  }

  async function save() {
    // Keep the display order (disable first, then modes), then append any
    // selected entries not in the available list (stale config).
    const list = available.filter((m) => selected.has(m));
    for (const m of selected) if (!list.includes(m)) list.push(m);
    try {
      if (inheriting) {
        await api.setQuickCycleInheritance(true);
        store.characterProfile()!.QuickCycleMode = "inherit";
      } else {
        await api.setQuickCycleModes(list);
        if (isDefault) store.config!.UI.QuickCycleModes = list;
        else {
          store.characterProfile()!.QuickCycleMode = "customize";
          store.characterProfile()!.QuickCycleModes = list;
        }
      }
      store.addToast("Quick-cycle modes", "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title="Quick-Cycle Modes (Alt+M)" back onsave={save}>
  {#if !isDefault}
    <div class="scope">
      <label><input type="radio" name="quick-scope" checked={inheriting} onchange={() => (inheriting = true)} /> Inherit Default</label>
      <label><input type="radio" name="quick-scope" checked={!inheriting} onchange={customize} /> Customize</label>
    </div>
  {/if}
  <p class="hint dim">Check the modes to include in the Alt+M rotation.</p>
  <div class="list">
    {#each available as mode (mode)}
      <label class="row">
        <input type="checkbox" checked={selected.has(mode)} onchange={() => toggle(mode)} disabled={inheriting} />
        <span class="name">{mode}</span>
      </label>
    {/each}
    {#if available.length <= 1}
      <div class="dim empty">No Lua modes loaded. Add script directories, then reload scripts.</div>
    {/if}
  </div>
</Modal>

<style>
  .hint {
    margin: 0 0 12px;
    font-size: 12px;
  }
  .scope { display: flex; gap: 18px; margin-bottom: 10px; font-size: 12px; }
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
