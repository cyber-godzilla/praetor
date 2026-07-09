<script lang="ts">
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  const sets = $derived(store.config?.UI?.ActionSets ?? []);

  // Component-local selection; resets to the first set each session (not persisted).
  let sel = $state(0);
  // Guard the index against set deletions/reorders from the editor.
  const current = $derived(sets.length ? sets[Math.min(sel, sets.length - 1)] : undefined);

  function fire(cmd: string) {
    api.send(cmd);
  }
</script>

{#if sets.length === 0}
  <button class="empty dim" onclick={() => (store.openModal = "actionsets")} tabindex="-1">
    No action sets yet — add one
  </button>
{:else}
  {#if sets.length > 1}
    <select class="setpicker" bind:value={sel} tabindex="-1">
      {#each sets as s, i (i)}
        <option value={i}>{s.Name}</option>
      {/each}
    </select>
  {/if}
  <div class="buttons">
    {#each current?.Buttons ?? [] as b (b.Label)}
      <button class="action" onclick={() => fire(b.Command)} title={b.Command} tabindex="-1">{b.Label}</button>
    {/each}
  </div>
{/if}

<style>
  .empty {
    display: block;
    width: 100%;
    text-align: left;
    font-size: 12px;
    padding: 8px 10px;
  }
  .setpicker {
    width: 100%;
    margin-bottom: 8px;
    font: inherit;
    font-size: 12px;
    padding: 4px 6px;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
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
</style>
