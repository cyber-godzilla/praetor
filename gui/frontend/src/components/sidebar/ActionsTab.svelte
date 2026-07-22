<script lang="ts">
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import { cycleIndex } from "../../lib/actions";

  const sets = $derived(store.config?.UI?.ActionSets ?? []);
  // Clamp the shown index defensively for the current render; the $effect below
  // corrects the stored value after sets shrink.
  const current = $derived(sets.length ? sets[Math.min(store.actionSetIndex, sets.length - 1)] : undefined);
  const canCycle = $derived(sets.length > 1);

  // Keep the shared index in range when sets are deleted/reordered in the editor.
  $effect(() => {
    const max = Math.max(0, sets.length - 1);
    if (store.actionSetIndex > max) store.actionSetIndex = max;
  });

  function cycle(delta: number) {
    if (!canCycle) return;
    store.actionSetIndex = cycleIndex(store.actionSetIndex, delta, sets.length);
  }
  async function fire(cmd: string) {
    try {
      await api.send(cmd);
    } catch (error) {
      store.addToast("Action unavailable", error instanceof Error ? error.message : String(error));
    }
  }
</script>

<div class="toolbar">
  <button class="nav" onclick={() => cycle(-1)} disabled={!canCycle} title="Previous set" aria-label="Previous set" tabindex="-1">‹</button>
  <button class="cur" onclick={() => (store.openModal = "actionsets")} title="Manage action sets" tabindex="-1">
    {current?.Name ?? "No sets"}
  </button>
  <button class="nav" onclick={() => cycle(1)} disabled={!canCycle} title="Next set" aria-label="Next set" tabindex="-1">›</button>
  <button class="add" onclick={() => (store.openModal = "actionset-add")} title="Add action set" aria-label="Add action set" tabindex="-1">+</button>
</div>

{#if current}
  <div class="buttons">
    {#each current.Buttons ?? [] as b, i (i)}
      <button class="action" onclick={() => fire(b.Command)} title={b.Command} tabindex="-1" disabled={!store.transportReady}>{b.Label}</button>
    {/each}
    <button class="action addbtn" onclick={() => (store.openModal = "action-add")} title="Add action" aria-label="Add action" tabindex="-1">+</button>
  </div>
{/if}

<style>
  .toolbar {
    display: flex;
    align-items: stretch;
    gap: 4px;
    margin-bottom: 8px;
  }
  .nav,
  .add {
    width: 26px;
    flex-shrink: 0;
    font-size: 14px;
    padding: 4px 0;
  }
  .nav:disabled {
    opacity: 0.35;
    cursor: default;
  }
  .cur {
    flex: 1;
    min-width: 0;
    font-size: 12px;
    padding: 4px 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .buttons {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }
  .action {
    padding: 8px 6px;
    font-size: 12px;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .addbtn {
    border-style: dashed;
  }
</style>
