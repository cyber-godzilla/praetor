<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import Frame from "./Frame.svelte";
  import SidebarVitals from "./SidebarVitals.svelte";
  import SidebarTabs from "./SidebarTabs.svelte";

  // Compass on-screen size scales with the user's compass scale (bigger scale =
  // bigger). Base 120px at scale 1.0; capped to the panel width by max-width.
  const compassPx = $derived(Math.round(120 * (store.config?.UI?.CompassScale ?? 1)));

  // Clicking a compass segment sends the movement command for that direction.
  function go(dir: string) {
    api.send(dir);
  }

  // Clicking the minimap sizes up the current area.
  function sizeup() {
    api.send("sizeup here");
  }
</script>

<div class="sidebar">
  <Frame title="Map">
    <div class="mapbox">
      {#if store.minimap}
        <button class="mapbtn" title="Size up the area" onclick={sizeup} tabindex="-1">
          <img src={store.minimap} alt="minimap" />
        </button>
      {:else}
        <div class="empty dim">no map data</div>
      {/if}
    </div>
  </Frame>

  <Frame title="Exits">
    <div class="compassbox">
      {#if store.compass}
        <div class="compass" style="width:{compassPx}px; height:{compassPx}px">
          <img src={store.compass} alt="compass" />
          <!-- Transparent 3×3 hotspot grid over the rose; center splits into
               up (top) / down (bottom). Click sends the movement command. -->
          <div class="hotspots">
            <button style="grid-area:nw" title="Northwest" aria-label="Go northwest" onclick={() => go("nw")} tabindex="-1"></button>
            <button style="grid-area:n" title="North" aria-label="Go north" onclick={() => go("n")} tabindex="-1"></button>
            <button style="grid-area:ne" title="Northeast" aria-label="Go northeast" onclick={() => go("ne")} tabindex="-1"></button>
            <button style="grid-area:w" title="West" aria-label="Go west" onclick={() => go("w")} tabindex="-1"></button>
            <div class="ud" style="grid-area:c">
              <button title="Up" aria-label="Go up" onclick={() => go("up")} tabindex="-1"></button>
              <button title="Down" aria-label="Go down" onclick={() => go("down")} tabindex="-1"></button>
            </div>
            <button style="grid-area:e" title="East" aria-label="Go east" onclick={() => go("e")} tabindex="-1"></button>
            <button style="grid-area:sw" title="Southwest" aria-label="Go southwest" onclick={() => go("sw")} tabindex="-1"></button>
            <button style="grid-area:s" title="South" aria-label="Go south" onclick={() => go("s")} tabindex="-1"></button>
            <button style="grid-area:se" title="Southeast" aria-label="Go southeast" onclick={() => go("se")} tabindex="-1"></button>
          </div>
        </div>
      {:else}
        <div class="empty dim">—</div>
      {/if}
    </div>
  </Frame>

  <SidebarVitals />

  <SidebarTabs />
</div>

<style>
  .sidebar {
    width: 260px;
    flex-shrink: 0;
    background: var(--bg-panel);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 12px 10px;
    overflow-y: auto;
  }
  .mapbox,
  .compassbox {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 56px;
  }
  .mapbtn {
    display: block;
    max-width: 100%;
    padding: 0;
    border: none;
    background: none;
    cursor: pointer;
    line-height: 0;
  }
  .mapbox img {
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
    padding: 0;
    border: none;
    background: transparent;
    cursor: pointer;
  }
  .hotspots button:hover {
    background: rgba(232, 168, 56, 0.18);
  }
  .ud {
    grid-area: c;
    display: grid;
    grid-template-rows: 1fr 1fr;
  }
  .empty {
    font-size: 12px;
    padding: 10px;
  }
</style>
