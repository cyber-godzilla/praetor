<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { PersistentKeyInfo } from "../../lib/types";

  let keys = $state<PersistentKeyInfo[]>([]);
  let selected = $state<Set<string>>(new Set());
  let notice = $state("");

  async function refresh() {
    keys = await api.getPersistentData();
  }
  $effect(() => {
    refresh();
  });

  function toggle(k: string) {
    const s = new Set(selected);
    if (s.has(k)) s.delete(k);
    else s.add(k);
    selected = s;
  }
  function selectedKeys(): string[] {
    return keys.map((k) => k.key).filter((k) => selected.has(k));
  }

  async function exportSel() {
    const sel = selectedKeys();
    if (sel.length === 0) return;
    const path = await api.exportPersistentData(sel);
    notice = "Exported to " + path;
    store.addToast("Persistent data", notice);
  }
  async function clearSel() {
    const sel = selectedKeys();
    if (sel.length === 0) return;
    await api.clearPersistentData(sel);
    selected = new Set();
    await refresh();
    notice = "Cleared " + sel.length + " key(s)";
  }
</script>

<Modal title="Persistent Lua Data" wide back>
  {#if notice}<p class="notice dim">{notice}</p>{/if}
  <div class="list">
    {#each keys as k (k.key)}
      <label class="krow">
        <input type="checkbox" checked={selected.has(k.key)} onchange={() => toggle(k.key)} />
        <span class="kname">{k.key}</span>
        <span class="spacer"></span>
        <span class="dim">{k.valueSummary}</span>
      </label>
    {/each}
    {#if keys.length === 0}<div class="dim empty">No persisted keys.</div>{/if}
  </div>
  {#snippet footer()}
    <button onclick={exportSel} disabled={selected.size === 0}>Export selected</button>
    <button class="danger" onclick={clearSel} disabled={selected.size === 0}>Clear selected</button>
  {/snippet}
</Modal>

<style>
  .notice {
    margin: 0 0 12px;
    font-size: 12px;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .krow {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 7px 10px;
    background: var(--bg-elevated);
    border-radius: 5px;
    font-size: 13px;
    cursor: pointer;
  }
  .kname {
    font-family: var(--mono);
  }
  .empty {
    padding: 20px;
    text-align: center;
  }
</style>
