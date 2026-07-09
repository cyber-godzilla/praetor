<script lang="ts">
  import { store } from "../../lib/store.svelte";
</script>

{#if store.displayState.length > 0}
  <div class="statelist">
    {#each store.displayState as item (item.label)}
      <div class="staterow">
        <span class="dim">{item.label}</span>
        <span class="val">{item.value}</span>
      </div>
    {/each}
  </div>
{:else}
  <div class="empty dim">no mode state</div>
{/if}

<style>
  .statelist {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .staterow {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
  }
  .empty {
    font-size: 12px;
    padding: 6px 2px;
  }
</style>
