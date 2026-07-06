<script lang="ts">
  import { store } from "../lib/store.svelte";

  // Compass on-screen size scales with the user's compass scale (bigger scale =
  // bigger). Base 120px at scale 1.0; capped to the panel width by max-width.
  const compassPx = $derived(Math.round(120 * (store.config?.UI?.CompassScale ?? 1)));
</script>

<div class="sidebar">
  <button class="modebtn" onclick={() => (store.openModal = "modeselect")} title="Switch mode">
    <span class="dim">Mode</span>
    <span class="modeval" class:on={!!store.mode && store.mode !== "disable"}>
      {store.mode && store.mode !== "disable" ? store.mode : "disable"}
    </span>
  </button>

  <div class="panel map">
    <div class="phead">Map</div>
    <div class="mapbox">
      {#if store.minimap}
        <img src={store.minimap} alt="minimap" />
      {:else}
        <div class="empty dim">no map data</div>
      {/if}
    </div>
  </div>

  <div class="panel compass">
    <div class="phead">Exits</div>
    <div class="compassbox">
      {#if store.compass}
        <img src={store.compass} alt="compass" style="width:{compassPx}px" />
      {:else}
        <div class="empty dim">—</div>
      {/if}
    </div>
  </div>

  {#if store.displayState.length > 0}
    <div class="panel state">
      <div class="phead">{store.mode || "state"}</div>
      <div class="statelist">
        {#each store.displayState as item (item.label)}
          <div class="staterow">
            <span class="dim">{item.label}</span>
            <span class="val">{item.value}</span>
          </div>
        {/each}
      </div>
    </div>
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
    gap: 10px;
    padding: 10px;
    overflow-y: auto;
  }
  .modebtn {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 9px 12px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    font-size: 13px;
  }
  .modebtn:hover {
    border-color: var(--accent);
  }
  .modeval {
    font-family: var(--mono);
    color: var(--fg-dim);
  }
  .modeval.on {
    color: var(--accent);
  }
  .panel {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .phead {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--accent);
    padding: 6px 10px;
    border-bottom: 1px solid var(--border);
  }
  .mapbox,
  .compassbox {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 10px;
    min-height: 60px;
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
    padding: 16px;
  }
  .statelist {
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .staterow {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
  }
  .val {
    font-family: var(--mono);
  }
</style>
