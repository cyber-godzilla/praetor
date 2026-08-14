<script lang="ts">
  import { onMount } from "svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import { modeList, isActiveMode } from "../../lib/modes";

  const modes = $derived(modeList(store.modeNames));

  // Refresh the mode list when the tab mounts so it reflects scripts loaded or
  // reloaded since startup (mirrors ModeSelectModal.onMount).
  onMount(async () => {
    const names = await api.modeNames();
    if (names && names.length) store.modeNames = names;
  });

  async function pick(mode: string) {
    try {
      await api.setMode(mode, []);
    } catch (e) {
      store.addToast("Mode error", String(e));
    }
  }
</script>

{#if (store.modeNames ?? []).length === 0}
  <div class="empty dim">No Lua modes loaded. Add script directories in the menu, then reload scripts.</div>
{/if}
<div class="modes">
  {#each modes as mode (mode)}
    <button class="mode" class:current={isActiveMode(mode, store.mode)} onclick={() => pick(mode)} tabindex="-1" disabled={!store.transportReady}>
      <span class="name">{mode}</span>
      {#if isActiveMode(mode, store.mode)}<span class="badge">active</span>{/if}
    </button>
  {/each}
</div>

<style>
  .empty {
    font-size: 12px;
    padding: 6px 2px 10px;
  }
  .modes {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .mode {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    text-align: left;
    font-size: 13px;
  }
  .mode.current {
    border-color: var(--accent);
    color: var(--accent);
  }
  .name {
    font-family: var(--mono);
  }
  .badge {
    font-size: 10px;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
</style>
