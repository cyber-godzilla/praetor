<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { vitalColor, vitalFillPct } from "../lib/vitals";

  let { compact = false }: { compact?: boolean } = $props();
  const bars = $derived([
    { label: "HP", value: store.health },
    { label: "FT", value: store.fatigue },
    { label: "EN", value: store.encumbrance },
    { label: "SA", value: store.satiation },
  ]);
</script>

<div class="gauges" class:compact>
  {#each bars as b (b.label)}
    <button class="gauge" title="{b.label} — click to check condition" onclick={() => api.send("cond")} tabindex="-1" disabled={!store.transportReady}>
      <span class="glabel dim">{b.label}</span>
      <span class="track"><span class="fill" style="height:{vitalFillPct(b.value)}%; background:{vitalColor(b.value)}"></span></span>
      <span class="gval" style="color:{vitalColor(b.value)}">{b.value ?? "—"}</span>
    </button>
  {/each}
</div>

<style>
  .gauges { width: 100%; height: 100%; display: flex; justify-content: space-around; align-items: center; gap: 14px; overflow: auto; }
  .gauges:not(.compact) .track { height: min(180px, 35vh); width: 24px; }
  .gauge { display: flex; flex-direction: column; align-items: center; gap: 4px; background: none; border: none; padding: 0; cursor: pointer; font: inherit; }
  .gauge:hover .glabel { color: var(--accent); }
  .glabel { font-size: 11px; }
  .track { position: relative; width: 14px; height: 52px; background: var(--bar-empty); border: 1px solid var(--border); display: block; }
  .fill { position: absolute; left: 0; bottom: 0; width: 100%; display: block; }
  .gval { font-size: 12px; }
</style>
