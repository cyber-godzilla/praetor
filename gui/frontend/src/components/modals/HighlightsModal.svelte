<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { HighlightConfig } from "../../lib/types";
  import { STYLE_COLORS } from "../../lib/highlight";

  const STYLES = ["red", "gold", "green", "blue"];
  function preview(style: string): string {
    const c = STYLE_COLORS[style] ?? STYLE_COLORS.gold;
    return `background:${c.bg};color:${c.fg}`;
  }

  let items = $state<HighlightConfig[]>(
    (store.config?.Highlights ?? []).map((h) => ({ ...h })),
  );
  let draft = $state("");

  // Reordering via up/down arrows (order sets highlight precedence).
  function move(i: number, dir: -1 | 1) {
    const j = i + dir;
    if (j < 0 || j >= items.length) return;
    const next = [...items];
    [next[i], next[j]] = [next[j], next[i]];
    items = next;
  }

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
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title="Highlights" wide back onsave={save}>
  <p class="hint dim">Case-insensitive substring matches, highlighted in the chosen color.</p>
  <div class="list">
    {#each items as item, i (i)}
      <div class="hl" class:off={!item.Active}>
        <span class="reorder">
          <button class="arrow" onclick={() => move(i, -1)} disabled={i === 0}
            title="Move up" aria-label="Move up">▲</button>
          <button class="arrow" onclick={() => move(i, 1)} disabled={i === items.length - 1}
            title="Move down" aria-label="Move down">▼</button>
        </span>
        <input type="checkbox" checked={item.Active} onchange={() => toggle(i)} title="Active" />
        <span class="pat" style={preview(item.Style)}>{item.Pattern}</span>
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
    padding: 2px;
    border: 1px solid transparent;
    border-radius: 6px;
  }
  .hl.off {
    opacity: 0.5;
  }
  .reorder {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .arrow {
    min-width: 0;
    padding: 0 5px;
    font-size: 9px;
    line-height: 1.2;
    color: var(--fg-dim);
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
