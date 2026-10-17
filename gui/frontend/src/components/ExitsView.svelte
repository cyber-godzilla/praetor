<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let { compact = false }: { compact?: boolean } = $props();
  const compassPx = $derived(compact ? 132 : Math.round(160 * (store.config?.UI?.CompassScale ?? 1)));

  function go(direction: string) {
    if (store.transportReady) api.send(direction);
  }
</script>

<div class="compassbox" class:compact>
  {#if store.compass}
    <div class="compass" style="width:{compassPx}px; height:{compassPx}px">
      <img src={store.compass} alt="compass" />
      <div class="hotspots">
        <button style="grid-area:nw" aria-label="Go northwest" onclick={() => go("nw")} tabindex="-1" disabled={!store.transportReady}></button>
        <button style="grid-area:n" aria-label="Go north" onclick={() => go("n")} tabindex="-1" disabled={!store.transportReady}></button>
        <button style="grid-area:ne" aria-label="Go northeast" onclick={() => go("ne")} tabindex="-1" disabled={!store.transportReady}></button>
        <button style="grid-area:w" aria-label="Go west" onclick={() => go("w")} tabindex="-1" disabled={!store.transportReady}></button>
        <div class="ud" style="grid-area:c">
          <button aria-label="Go up" onclick={() => go("up")} tabindex="-1" disabled={!store.transportReady}></button>
          <button aria-label="Go down" onclick={() => go("down")} tabindex="-1" disabled={!store.transportReady}></button>
        </div>
        <button style="grid-area:e" aria-label="Go east" onclick={() => go("e")} tabindex="-1" disabled={!store.transportReady}></button>
        <button style="grid-area:sw" aria-label="Go southwest" onclick={() => go("sw")} tabindex="-1" disabled={!store.transportReady}></button>
        <button style="grid-area:s" aria-label="Go south" onclick={() => go("s")} tabindex="-1" disabled={!store.transportReady}></button>
        <button style="grid-area:se" aria-label="Go southeast" onclick={() => go("se")} tabindex="-1" disabled={!store.transportReady}></button>
      </div>
    </div>
  {:else}
    <div class="empty dim">—</div>
  {/if}
</div>

<style>
  .compassbox { width: 100%; height: 100%; min-height: 56px; display: flex; align-items: center; justify-content: center; overflow: auto; }
  .compass { position: relative; max-width: 100%; max-height: 100%; }
  img { display: block; width: 100%; height: 100%; image-rendering: auto; }
  .hotspots { position: absolute; inset: 0; display: grid; grid-template-columns: repeat(3, 1fr); grid-template-rows: repeat(3, 1fr); grid-template-areas: "nw n ne" "w c e" "sw s se"; }
  .hotspots button { min-width: 0; min-height: 0; padding: 0; border: none; background: transparent; cursor: pointer; }
  .hotspots button:hover, .hotspots button:active { background: rgba(232, 168, 56, 0.18); }
  .ud { display: grid; grid-template-rows: 1fr 1fr; }
  .empty { font-size: 12px; padding: 10px; }
</style>
