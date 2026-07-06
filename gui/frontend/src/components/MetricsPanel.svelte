<script lang="ts">
  import { store } from "../lib/store.svelte";
  import type { MetricSession } from "../lib/types";

  function fmtDur(ms: number): string {
    const s = Math.floor(ms / 1000);
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    if (h > 0) return `${h}h ${m}m ${sec}s`;
    if (m > 0) return `${m}m ${sec}s`;
    return `${sec}s`;
  }

  const current = $derived<MetricSession | undefined>(store.status?.current);
  const history = $derived<MetricSession[]>(store.status?.history ?? []);
</script>

<div class="metrics">
  {#if current}
    <div class="card">
      <div class="chead">
        <span class="mode">{current.mode || "session"}</span>
        <span class="dim">active · {fmtDur(current.durationMs)}</span>
      </div>
      <div class="entries">
        {#each current.entries as e (e.label)}
          <div class="entry">
            <span class="v">{e.value}</span>
            <span class="dim">{e.label}</span>
          </div>
        {/each}
        {#if current.entries.length === 0}
          <div class="dim">no metrics tracked this session</div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="dim empty">No active metrics session.</div>
  {/if}

  {#if history.length > 0}
    <div class="hhead dim">History</div>
    {#each history.slice().reverse() as h, i (i)}
      <div class="card small">
        <div class="chead">
          <span class="mode">{h.mode || "session"}</span>
          <span class="dim">{fmtDur(h.durationMs)}</span>
        </div>
        <div class="entries">
          {#each h.entries as e (e.label)}
            <div class="entry"><span class="v">{e.value}</span><span class="dim">{e.label}</span></div>
          {/each}
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .metrics {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .card {
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 14px;
  }
  .card.small {
    padding: 10px 14px;
  }
  .chead {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 10px;
  }
  .mode {
    color: var(--accent);
    font-weight: 600;
  }
  .entries {
    display: flex;
    flex-wrap: wrap;
    gap: 18px;
  }
  .entry {
    display: flex;
    flex-direction: column;
  }
  .entry .v {
    font-size: 22px;
    font-family: var(--mono);
    font-weight: 700;
  }
  .hhead {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-top: 6px;
  }
  .empty {
    padding: 24px;
    text-align: center;
  }
</style>
