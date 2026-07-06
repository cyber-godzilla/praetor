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

  // Drag-and-drop reordering.
  let dragIndex = $state<number | null>(null);
  let overIndex = $state<number | null>(null);

  function onDragStart(i: number) {
    dragIndex = i;
  }
  function onDragOver(i: number, e: DragEvent) {
    e.preventDefault();
    overIndex = i;
  }
  function onDrop(i: number, e: DragEvent) {
    e.preventDefault();
    if (dragIndex === null || dragIndex === i) {
      dragIndex = overIndex = null;
      return;
    }
    const next = [...items];
    const [moved] = next.splice(dragIndex, 1);
    next.splice(i, 0, moved);
    items = next;
    dragIndex = overIndex = null;
  }
  function onDragEnd() {
    dragIndex = overIndex = null;
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
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="Highlights" wide back>
  <p class="hint dim">Case-insensitive substring matches, highlighted in the chosen color.</p>
  <div class="list">
    {#each items as item, i (i)}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="hl"
        class:off={!item.Active}
        class:dragging={dragIndex === i}
        class:over={overIndex === i && dragIndex !== i}
        ondragover={(e) => onDragOver(i, e)}
        ondrop={(e) => onDrop(i, e)}
      >
        <span
          class="handle"
          draggable="true"
          ondragstart={() => onDragStart(i)}
          ondragend={onDragEnd}
          title="Drag to reorder"
          role="button"
          tabindex="-1"
          aria-label="Drag to reorder">⠿</span>
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
    padding: 2px;
    border: 1px solid transparent;
    border-radius: 6px;
  }
  .hl.off {
    opacity: 0.5;
  }
  .hl.dragging {
    opacity: 0.4;
  }
  .hl.over {
    border-color: var(--accent);
    background: rgba(232, 168, 56, 0.08);
  }
  .handle {
    cursor: grab;
    color: var(--fg-dim);
    font-size: 15px;
    padding: 0 2px;
    user-select: none;
  }
  .handle:active {
    cursor: grabbing;
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
