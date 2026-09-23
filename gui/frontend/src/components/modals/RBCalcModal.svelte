<script lang="ts">
  import Modal from "../Modal.svelte";
  import * as api from "../../lib/bridge";
  import type { RBResult, TrainingCostRow } from "../../lib/types";

  const MODES = ["Defensive", "Offensive", "Noncombat"];
  const DIFFS = ["Basic", "Easy", "Average", "Difficult", "Impossible"];
  const POSTURES = ["Berserk", "Aggressive", "Normal", "Wary", "Defensive"];

  let mode = $state(0);
  let curBasics = $state(0);
  let curSub = $state(0);
  let tgtBasics = $state(0);
  let tgtSub = $state(0);
  let selfTrained = $state(false);
  let selfTaught = $state(false);
  let healing = $state(false);
  let slotPage = $state(0);
  let current = $state<RBResult | null>(null);
  let target = $state<RBResult | null>(null);
  let costs = $state<TrainingCostRow[]>([]);
  let generation = 0;

  $effect(() => {
    const request = ++generation;
    const currentArgs = [mode, curBasics || 0, curSub || 0] as const;
    const targetArgs = [mode, tgtBasics || 0, tgtSub || 0] as const;
    const costArgs = [
      curBasics || 0,
      curSub || 0,
      tgtBasics || 0,
      tgtSub || 0,
      selfTrained,
      selfTaught,
      healing,
    ] as const;
    Promise.all([
      api.calcRankBonus(...currentArgs),
      api.calcRankBonus(...targetArgs),
      api.calcTrainingCosts(...costArgs),
    ]).then(([cur, tgt, rows]) => {
      if (request !== generation) return;
      current = cur;
      target = tgt;
      costs = rows;
    });
  });

  const rows = $derived(mode === 2 ? [0] : mode === 0 ? [4, 3, 2, 1, 0] : [0, 1, 2, 3, 4]);
  const visibleCosts = $derived(costs.slice(slotPage * 10, slotPage * 10 + 10));

  function cell(result: RBResult, posture: number, difficulty: number): number {
    return result.cells.find((x) => x.posture === posture && x.difficulty === difficulty)?.bonus ?? 0;
  }

  function format(value: number): string {
    return Number.isInteger(value) ? String(value) : value.toFixed(1);
  }
</script>

<Modal title="Rank-Bonus Calculator" extraWide back>
  <div class="inputs">
    <label>Mode
      <select bind:value={mode}>{#each MODES as m, i (m)}<option value={i}>{m}</option>{/each}</select>
    </label>
    <fieldset>
      <legend>Current ranks</legend>
      <label>Basics <input aria-label="Current basics" type="number" min="0" bind:value={curBasics} /></label>
      <label>Subskill <input aria-label="Current subskill" type="number" min="0" bind:value={curSub} /></label>
    </fieldset>
    <fieldset>
      <legend>Target ranks</legend>
      <label>Basics <input aria-label="Target basics" type="number" min="0" bind:value={tgtBasics} /></label>
      <label>Subskill <input aria-label="Target subskill" type="number" min="0" bind:value={tgtSub} /></label>
    </fieldset>
  </div>

  <div class="comparisons">
    {#if current}{@render RankTable("Current", current)}{/if}
    {#if target && tgtBasics > 0 && tgtSub > 0}{@render RankTable("Target", target)}{/if}
  </div>

  {#if tgtSub > 0}
    <div class="training-head">
      <div><span class="accent">Training cost</span> · ΔBasics {tgtBasics - curBasics >= 0 ? "+" : ""}{tgtBasics - curBasics} · ΔSubskill {tgtSub - curSub >= 0 ? "+" : ""}{tgtSub - curSub}</div>
      <button class="sm" onclick={() => (slotPage = 1 - slotPage)}>Show slots {slotPage === 0 ? "11–20" : "1–10"}</button>
    </div>
    <div class="flags">
      <label><input type="checkbox" bind:checked={selfTrained} /> Self-trained</label>
      <label><input type="checkbox" bind:checked={selfTaught} /> Self-taught</label>
      <label><input type="checkbox" bind:checked={healing} /> Healing</label>
    </div>
    <table class="matrix">
      <thead><tr><th>Slot</th>{#each DIFFS as d (d)}<th>{d}</th>{/each}</tr></thead>
      <tbody>
        {#each visibleCosts as row (row.slot)}
          <tr><td class="accent">{row.slot}</td><td>{row.basic}</td><td>{row.easy}</td><td>{row.average}</td><td>{row.difficult}</td><td>{row.impossible}</td></tr>
        {/each}
      </tbody>
    </table>
    {#if (tgtBasics > 1150 || tgtSub > 1150) && !selfTrained}
      <div class="warning">Ranks 1151+ require /selftrain; shown costs already reflect that rule.</div>
    {/if}
  {/if}
</Modal>

{#snippet RankTable(title: string, result: RBResult)}
  <section class="rank-table">
    <div class="table-title">{title}</div>
    <table>
      <thead><tr><th>Posture</th>{#each DIFFS as d (d)}<th>{d}</th>{/each}</tr></thead>
      <tbody>
        {#each rows as posture (posture)}
          <tr>
            <td class="accent">{mode === 2 ? "Noncombat" : POSTURES[posture]}</td>
            {#each DIFFS as _d, difficulty (difficulty)}<td>{format(cell(result, posture, difficulty))}</td>{/each}
          </tr>
        {/each}
      </tbody>
    </table>
    <div class="tier">Basics RB {format(result.basicsRB)} · Subskill RB {format(result.subskillRB)}</div>
  </section>
{/snippet}

<style>
  .inputs { display: grid; grid-template-columns: auto 1fr 1fr; gap: 12px; align-items: end; }
  label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--fg-dim); }
  fieldset { display: flex; gap: 10px; border: 1px solid var(--border); padding: 6px 10px 9px; }
  legend { color: var(--fg-dim); font-size: 11px; }
  input[type="number"] { width: 100%; min-width: 70px; }
  .comparisons { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 14px; }
  .rank-table { min-width: 0; }
  .rank-table table { table-layout: fixed; }
  .rank-table th { overflow-wrap: anywhere; }
  .rank-table th:first-child { width: 82px; }
  .table-title, .accent { color: var(--accent); }
  .table-title { margin-bottom: 4px; font-weight: 700; }
  table { width: 100%; border-collapse: collapse; font-size: 12px; }
  th, td { border: 1px solid var(--border); padding: 4px 5px; text-align: right; }
  th { color: var(--fg-dim); font-weight: 500; }
  th:first-child, td:first-child { text-align: left; }
  .tier { color: var(--fg-dim); font-size: 11px; margin-top: 5px; }
  .training-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-top: 18px; }
  .flags { display: flex; gap: 18px; margin: 8px 0; }
  .flags label { flex-direction: row; align-items: center; color: var(--fg); font-size: 13px; }
  .warning { margin-top: 8px; color: var(--green); font-size: 12px; }
  .sm { padding: 3px 8px; font-size: 11px; }
  @media (max-width: 850px) { .comparisons { grid-template-columns: 1fr; } }
  @media (max-width: 650px) { .inputs { grid-template-columns: 1fr; } }
</style>
