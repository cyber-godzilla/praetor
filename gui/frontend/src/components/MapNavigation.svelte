<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import Frame from "./Frame.svelte";

  let { compact = false }: { compact?: boolean } = $props();
  // Keep the touch compass at a fixed 132px (44px per hotspot). The persisted
  // scale remains a desktop presentation setting; allowing it to shrink the
  // mobile compass would create inaccessible movement targets.
  const compassPx = $derived(compact ? 132 : Math.round(120 * (store.config?.UI?.CompassScale ?? 1)));

  function go(direction: string) {
    if (!store.transportReady) return;
    api.send(direction);
  }

  function sizeup() {
    if (!store.transportReady) return;
    api.send("sizeup here");
  }
</script>

<div class="navigation" class:compact>
  <Frame title="Map" collapsible={!compact} bind:collapsed={store.collapsed.map}>
    <div class="mapbox">
      {#if store.minimap}
        <button class="mapbtn" title="Size up the area" aria-label="Size up the current area" onclick={sizeup} tabindex="-1" disabled={!store.transportReady}>
          <img src={store.minimap} alt="minimap" />
        </button>
      {:else}
        <div class="empty dim">no map data</div>
      {/if}
    </div>
  </Frame>

  <Frame title="Exits" collapsible={!compact} bind:collapsed={store.collapsed.exits}>
    <div class="compassbox">
      {#if store.compass}
        <div class="compass" style="width:{compassPx}px; height:{compassPx}px">
          <img src={store.compass} alt="compass" />
          <div class="hotspots">
            <button style="grid-area:nw" title="Northwest" aria-label="Go northwest" onclick={() => go("nw")} tabindex="-1" disabled={!store.transportReady}></button>
            <button style="grid-area:n" title="North" aria-label="Go north" onclick={() => go("n")} tabindex="-1" disabled={!store.transportReady}></button>
            <button style="grid-area:ne" title="Northeast" aria-label="Go northeast" onclick={() => go("ne")} tabindex="-1" disabled={!store.transportReady}></button>
            <button style="grid-area:w" title="West" aria-label="Go west" onclick={() => go("w")} tabindex="-1" disabled={!store.transportReady}></button>
            <div class="ud" style="grid-area:c">
              <button title="Up" aria-label="Go up" onclick={() => go("up")} tabindex="-1" disabled={!store.transportReady}></button>
              <button title="Down" aria-label="Go down" onclick={() => go("down")} tabindex="-1" disabled={!store.transportReady}></button>
            </div>
            <button style="grid-area:e" title="East" aria-label="Go east" onclick={() => go("e")} tabindex="-1" disabled={!store.transportReady}></button>
            <button style="grid-area:sw" title="Southwest" aria-label="Go southwest" onclick={() => go("sw")} tabindex="-1" disabled={!store.transportReady}></button>
            <button style="grid-area:s" title="South" aria-label="Go south" onclick={() => go("s")} tabindex="-1" disabled={!store.transportReady}></button>
            <button style="grid-area:se" title="Southeast" aria-label="Go southeast" onclick={() => go("se")} tabindex="-1" disabled={!store.transportReady}></button>
          </div>
        </div>
        {#if compact}
          <div class="vertical-controls" aria-label="Vertical navigation">
            <button title="Up" aria-label="Go up" onclick={() => go("up")} disabled={!store.transportReady}>↑<span>UP</span></button>
            <button title="Down" aria-label="Go down" onclick={() => go("down")} disabled={!store.transportReady}>↓<span>DN</span></button>
          </div>
        {/if}
      {:else}
        <div class="empty dim">—</div>
      {/if}
    </div>
  </Frame>
</div>

<style>
  .navigation {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .navigation.compact {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 206px;
    align-items: stretch;
    gap: 8px;
  }
  .mapbox,
  .compassbox {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 56px;
  }
  .compact .mapbox,
  .compact .compassbox {
    height: 136px;
    min-height: 136px;
    overflow: hidden;
  }
  .mapbtn {
    display: block;
    max-width: 100%;
    max-height: 100%;
    padding: 0;
    border: none;
    background: none;
    cursor: pointer;
    line-height: 0;
  }
  .mapbox img {
    display: block;
    max-width: 100%;
    image-rendering: pixelated;
  }
  .compass {
    position: relative;
    max-width: 100%;
  }
  .compass img {
    display: block;
    width: 100%;
    height: 100%;
    image-rendering: auto;
  }
  .hotspots {
    position: absolute;
    inset: 0;
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    grid-template-rows: 1fr 1fr 1fr;
    grid-template-areas:
      "nw n  ne"
      "w  c  e"
      "sw s  se";
  }
  .hotspots button {
    min-width: 0;
    min-height: 0;
    padding: 0;
    border: none;
    background: transparent;
    cursor: pointer;
  }
  .hotspots button:hover,
  .hotspots button:active {
    background: rgba(232, 168, 56, 0.18);
  }
  .ud {
    display: grid;
    grid-template-rows: 1fr 1fr;
  }
  .vertical-controls {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-left: 4px;
  }
  .vertical-controls button {
    width: 44px;
    min-width: 44px;
    height: 44px;
    min-height: 44px;
    padding: 0;
    color: var(--accent);
    font-weight: 700;
  }
  .vertical-controls span {
    display: block;
    color: var(--fg-dim);
    font-size: 9px;
    line-height: 1;
  }
  .compact .ud {
    pointer-events: none;
  }
  .compact .mapbox img {
    max-height: 108px;
  }
  .empty {
    font-size: 12px;
    padding: 10px;
  }

  @media (max-width: 599px) {
    .navigation.compact {
      grid-template-columns: minmax(0, 1fr) 206px;
    }
    .compact .mapbox,
    .compact .compassbox {
      height: 136px;
      min-height: 136px;
    }
  }

  @media (max-width: 399px) {
    .navigation.compact {
      grid-template-columns: minmax(0, 1fr) 206px;
    }
  }
</style>
