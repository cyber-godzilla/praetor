<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import Frame from "./Frame.svelte";
  import { vitalColor, vitalFillPct } from "../lib/vitals";

  const bars = $derived([
    { label: "HP", value: store.health },
    { label: "FT", value: store.fatigue },
    { label: "EN", value: store.encumbrance },
    { label: "SA", value: store.satiation },
  ]);

  // Clicking any gauge checks condition, matching the top status bar.
  function checkCond() {
    api.send("cond");
  }
</script>

<Frame title="Vitals" collapsible bind:collapsed={store.collapsed.vitals}>
  <div class="gauges">
    {#each bars as b (b.label)}
      <button class="gauge" title="{b.label} — click to check condition" onclick={checkCond} tabindex="-1">
        <span class="glabel dim">{b.label}</span>
        <span class="track">
          <span class="fill" style="height:{vitalFillPct(b.value)}%; background:{vitalColor(b.value)}"></span>
        </span>
        <span class="gval" style="color:{vitalColor(b.value)}">{b.value ?? "—"}</span>
      </button>
    {/each}
  </div>
</Frame>

<style>
  .gauges {
    display: flex;
    justify-content: space-around;
    align-items: flex-end;
    gap: 6px;
  }
  .gauge {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    font: inherit;
  }
  .gauge:hover .glabel {
    color: var(--accent);
  }
  .glabel {
    font-size: 11px;
  }
  .track {
    position: relative;
    width: 14px;
    height: 52px;
    background: var(--bar-empty);
    border: 1px solid var(--border);
    display: block;
  }
  .fill {
    position: absolute;
    left: 0;
    bottom: 0;
    width: 100%;
    display: block;
  }
  .gval {
    font-size: 12px;
  }
</style>
