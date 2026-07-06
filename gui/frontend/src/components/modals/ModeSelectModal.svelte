<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  const modes = $derived(["disable", ...(store.modeNames ?? [])]);

  // Pull the current mode list on open so it reflects any scripts loaded/
  // reloaded since startup, rather than the initial snapshot.
  onMount(async () => {
    const names = await api.modeNames();
    if (names && names.length) store.modeNames = names;
  });

  async function pick(mode: string) {
    try {
      await api.setMode(mode, []);
      store.openModal = null;
    } catch (e) {
      store.addToast("Mode error", String(e));
    }
  }
</script>

<Modal title="Switch Mode">
  {#if (store.modeNames ?? []).length === 0}
    <p class="dim empty">No Lua modes loaded. Add script directories in the menu, then reload scripts.</p>
  {/if}
  <div class="modes">
    {#each modes as mode (mode)}
      <button class="mode" class:current={store.mode === mode || (mode === "disable" && (store.mode === "" || store.mode === "disable"))} onclick={() => pick(mode)}>
        <span class="name">{mode}</span>
        {#if store.mode === mode || (mode === "disable" && (store.mode === "" || store.mode === "disable"))}
          <span class="badge">active</span>
        {/if}
      </button>
    {/each}
  </div>
</Modal>

<style>
  .empty {
    margin: 0 0 12px;
    font-size: 13px;
  }
  .modes {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .mode {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 11px 14px;
    text-align: left;
    font-size: 14px;
  }
  .mode.current {
    border-color: var(--accent);
    color: var(--accent);
  }
  .name {
    font-family: var(--mono);
  }
  .badge {
    font-size: 11px;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
</style>
