<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { HighlightConfig } from "../../lib/types";

  const STYLES = ["red", "gold", "green", "blue"];
  const PREVIEW: Record<string, string> = {
    red: "background:#e05c5c;color:#fff",
    gold: "background:#e8a838;color:#000",
    green: "background:#6cc46c;color:#000",
    blue: "background:#5c8ce0;color:#fff",
  };

  let items = $state<HighlightConfig[]>(
    (store.config?.Highlights ?? []).map((h) => ({ ...h })),
  );
  let draft = $state("");

  function add() {
    const p = draft.trim();
    if (!p) return;
    items = [...items, { Pattern: p, Style: "gold", Active: true }];
    draft = "";
  }
  function remove(i: number) {
    items = items.filter((_, idx) => idx !== i);
  }
  function cycleStyle(i: number) {
    const cur = STYLES.indexOf(items[i].Style);
    items[i].Style = STYLES[(cur + 1) % STYLES.length];
  }
  function toggle(i: number) {
    items[i].Active = !items[i].Active;
  }

  async function save() {
    try {
      await api.setHighlights(items);
      store.config!.Highlights = items;
      store.addToast("Highlights", "Saved");
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="Highlights" wide>
  <p class="hint dim">Case-insensitive substring matches, highlighted in the chosen color.</p>
  <div class="list">
    {#each items as item, i (i)}
      <div class="hl" class:off={!item.Active}>
        <input type="checkbox" checked={item.Active} onchange={() => toggle(i)} title="Active" />
        <span class="pat" style={PREVIEW[item.Style]}>{item.Pattern}</span>
        <button class="sm" onclick={() => cycleStyle(i)}>{item.Style}</button>
        <button class="danger sm" onclick={() => remove(i)}>✕</button>
      </div>
    {/each}
    {#if items.length === 0}<div class="dim empty">No highlights yet.</div>{/if}
  </div>
  <div class="row add">
    <input type="text" bind:value={draft} placeholder="New pattern…"
      onkeydown={(e) => e.key === "Enter" && add()} />
    <button onclick={add}>Add</button>
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
    gap: 6px;
    margin-bottom: 12px;
  }
  .hl {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .hl.off {
    opacity: 0.5;
  }
  .pat {
    flex: 1;
    padding: 4px 10px;
    border-radius: 4px;
    font-family: var(--mono);
    font-size: 13px;
  }
  .sm {
    padding: 4px 10px;
    font-size: 12px;
    min-width: 54px;
  }
  .empty {
    padding: 14px;
    text-align: center;
  }
  .add input {
    flex: 1;
  }
</style>
