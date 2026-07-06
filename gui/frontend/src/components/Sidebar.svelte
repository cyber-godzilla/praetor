<script lang="ts">
  import { store } from "../lib/store.svelte";
  import Frame from "./Frame.svelte";

  // Compass on-screen size scales with the user's compass scale (bigger scale =
  // bigger). Base 120px at scale 1.0; capped to the panel width by max-width.
  const compassPx = $derived(Math.round(120 * (store.config?.UI?.CompassScale ?? 1)));
  const modeName = $derived(store.mode && store.mode !== "disable" ? store.mode : "disable");
</script>

<div class="sidebar">
  <button class="modebtn" onclick={() => (store.openModal = "modeselect")} title="Switch mode">
    <span class="dim">MODE</span>
    <span class="modeval" class:on={modeName !== "disable"}>{modeName}</span>
  </button>

  <Frame title="Map">
    <div class="mapbox">
      {#if store.minimap}
        <img src={store.minimap} alt="minimap" />
      {:else}
        <div class="empty dim">no map data</div>
      {/if}
    </div>
  </Frame>

  <Frame title="Exits">
    <div class="compassbox">
      {#if store.compass}
        <img src={store.compass} alt="compass" style="width:{compassPx}px" />
      {:else}
        <div class="empty dim">—</div>
      {/if}
    </div>
  </Frame>

  {#if store.displayState.length > 0}
    <Frame title={modeName}>
      <div class="statelist">
        {#each store.displayState as item (item.label)}
          <div class="staterow">
            <span class="dim">{item.label}</span>
            <span class="val">{item.value}</span>
          </div>
        {/each}
      </div>
    </Frame>
  {/if}
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
  .modebtn {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    background: var(--bg);
    border: 1px solid var(--border);
    font-size: 13px;
  }
  .modebtn:hover {
    border-color: var(--accent);
  }
  .modeval {
    color: var(--fg-dim);
  }
  .modeval.on {
    color: var(--accent);
  }
  .mapbox,
  .compassbox {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 56px;
  }
  .mapbox img {
    max-width: 100%;
    image-rendering: pixelated;
  }
  .compassbox img {
    max-width: 100%;
    height: auto;
    image-rendering: auto;
  }
  .empty {
    font-size: 12px;
    padding: 10px;
  }
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
</style>
