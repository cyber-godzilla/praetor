<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let { compact = false }: { compact?: boolean } = $props();

  function sizeup() {
    if (store.transportReady) api.send("sizeup here");
  }
</script>

<div class="mapbox" class:compact>
  {#if store.minimap}
    <button class="mapbtn" title="Size up the area" aria-label="Size up the current area" onclick={sizeup} tabindex="-1" disabled={!store.transportReady}>
      <img src={store.minimap} alt="minimap" />
    </button>
  {:else}
    <div class="empty dim">no map data</div>
  {/if}
</div>

<style>
  .mapbox { width: 100%; height: 100%; min-height: 56px; display: flex; align-items: center; justify-content: center; overflow: auto; }
  .mapbox.compact { min-height: 120px; }
  .mapbtn { display: block; max-width: 100%; max-height: 100%; padding: 0; border: none; background: none; cursor: pointer; line-height: 0; }
  img { display: block; max-width: 100%; max-height: 100%; image-rendering: pixelated; }
  .empty { font-size: 12px; padding: 10px; }
</style>
