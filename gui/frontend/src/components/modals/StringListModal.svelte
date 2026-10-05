<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";

  let {
    title,
    hint,
    initial,
    onsave,
    onBrowse,
  }: {
    title: string;
    hint?: string;
    initial: string[];
    onsave: (values: string[]) => Promise<void>;
    // Optional native folder picker. When provided, a "Browse…" button appears
    // and its non-empty result is appended (deduped). Other users of this modal
    // (ignore lists, high-priority commands) omit it and show no button.
    onBrowse?: () => Promise<string | null>;
  } = $props();

  // Seed once from the prop; the editor owns the list thereafter.
  let items = $state<string[]>(untrack(() => [...(initial ?? [])]));
  let draft = $state("");

  function add() {
    const v = draft.trim();
    if (v && !items.includes(v)) items = [...items, v];
    draft = "";
  }
  function remove(i: number) {
    items = items.filter((_, idx) => idx !== i);
  }
  async function browse() {
    if (!onBrowse) return;
    const v = await onBrowse();
    if (v && !items.includes(v)) items = [...items, v];
  }

  async function save() {
    try {
      await onsave(items);
      store.addToast(title, "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal {title} back onsave={save}>
  {#if hint}<p class="hint dim">{hint}</p>{/if}
  <div class="list">
    {#each items as item, i (item)}
      <div class="item">
        <span class="txt">{item}</span>
        <button class="danger sm" onclick={() => remove(i)}>Remove</button>
      </div>
    {/each}
    {#if items.length === 0}<div class="dim empty">Empty</div>{/if}
  </div>
  <div class="add row">
    <input type="text" bind:value={draft} placeholder="Add…"
      onkeydown={(e) => e.key === "Enter" && add()} />
    <button onclick={add}>Add</button>
    {#if onBrowse}<button onclick={browse}>Browse…</button>{/if}
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
    gap: 4px;
    margin-bottom: 12px;
  }
  .item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 7px 10px;
    background: var(--bg-elevated);
    border-radius: 5px;
  }
  .txt {
    font-family: var(--mono);
    font-size: 13px;
  }
  .sm {
    padding: 3px 8px;
    font-size: 12px;
  }
  .empty {
    padding: 12px;
    text-align: center;
  }
  .add input {
    flex: 1;
  }
</style>
