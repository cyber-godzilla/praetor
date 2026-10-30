<script lang="ts">
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import { cycleIndex } from "../../lib/actions";

  let { compact = false }: { compact?: boolean } = $props();

  const sets = $derived(store.resolvedActionSets());
  const current = $derived(sets.find((set) => set.ID === store.selectedActionSetID) ?? sets[0]);
  const canCycle = $derived(sets.length > 1);

  // Repair a missing legacy/deleted selection without coupling it to list order.
  $effect(() => {
    const fallback = sets[0]?.ID ?? "";
    if (!sets.some((set) => set.ID === store.selectedActionSetID)) {
      store.selectedActionSetID = fallback;
    }
  });

  async function cycle(delta: number) {
    if (!canCycle) return;
    const currentIndex = Math.max(0, sets.findIndex((set) => set.ID === current?.ID));
    const next = sets[cycleIndex(currentIndex, delta, sets.length)];
    if (!next) return;
    store.selectedActionSetID = next.ID;
    store.characterProfile()!.SelectedActionSetID = next.ID;
    try {
      await api.setSelectedActionSetID(next.ID);
    } catch (e) {
      store.addToast("Action Set", String(e));
    }
  }
  async function fire(cmd: string) {
    try {
      await api.sendInput(cmd);
    } catch (e) {
      store.addToast("Action error", String(e));
    }
  }
</script>

<div class="toolbar">
  <button class="nav" onclick={() => void cycle(-1)} disabled={!canCycle} title="Previous set" aria-label="Previous set" tabindex="-1">‹</button>
  <button class="cur" onclick={() => (store.openModal = "actionsets")} title="Manage action sets" tabindex="-1">
    {current?.Name ?? "No sets"}
  </button>
  <button class="nav" onclick={() => void cycle(1)} disabled={!canCycle} title="Next set" aria-label="Next set" tabindex="-1">›</button>
  <button class="add" onclick={() => (store.openModal = "actionset-add")} title="Add action set" aria-label="Add action set" tabindex="-1">+</button>
</div>

{#if current}
  <div class="buttons" class:responsive={compact}>
    {#each current.Buttons ?? [] as b, i (i)}
      <button class="action" onclick={() => void fire(b.Command)} title={b.Command} tabindex="-1" disabled={!store.transportReady}>{b.Label}</button>
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
  .buttons.responsive {
    grid-template-columns: repeat(auto-fit, minmax(96px, 1fr));
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
