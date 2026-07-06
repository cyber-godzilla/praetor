<script lang="ts">
  import Modal from "../Modal.svelte";
  import * as api from "../../lib/bridge";
  import type { RBResult } from "../../lib/types";

  const MODES = ["Defensive", "Offensive", "Noncombat"];
  const POSTURES = ["Berserk", "Aggressive", "Normal", "Wary", "Defensive"];
  const DIFFS = ["Basic", "Easy", "Average", "Difficult", "Impossible"];

  let mode = $state(0);
  let basics = $state(0);
  let subskill = $state(0);
  let result = $state<RBResult | null>(null);

  $effect(() => {
    (async () => {
      result = await api.calcRankBonus(mode, basics || 0, subskill || 0);
    })();
  });

  function cell(posture: number, difficulty: number): number {
    if (!result) return 0;
    const c = result.cells.find((x) => x.posture === posture && x.difficulty === difficulty);
    return c ? c.bonus : 0;
  }

  // Training cost mini-calculator.
  let curRank = $state(0);
  let desRank = $state(1);
  let slot = $state(1);
  let tdiff = $state(2);
  let selfTrained = $state(false);
  let selfTaught = $state(false);
  let healing = $state(false);
  let cost = $state(0);

  $effect(() => {
    (async () => {
      cost = await api.calcTrainCost(curRank || 0, desRank || 0, slot || 1, tdiff, selfTrained, selfTaught, healing);
    })();
  });
</script>

<Modal title="Rank-Bonus Calculator" wide>
  <div class="controls row">
    <label>Mode
      <select bind:value={mode}>
        {#each MODES as m, i (m)}<option value={i}>{m}</option>{/each}
      </select>
    </label>
    <label>Basics <input type="number" min="0" bind:value={basics} /></label>
    <label>Subskill <input type="number" min="0" bind:value={subskill} /></label>
  </div>

  {#if result}
    <table class="rb">
      <thead>
        <tr>
          <th>Posture</th>
          {#each DIFFS as d (d)}<th>{d}</th>{/each}
        </tr>
      </thead>
      <tbody>
        {#each POSTURES as p, pi (p)}
          <tr>
            <td class="ph">{p}</td>
            {#each DIFFS as _d, di (di)}
              <td>{cell(pi, di).toFixed(1)}</td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
    <div class="tier dim">Basics RB {result.basicsRB.toFixed(1)} · Subskill RB {result.subskillRB.toFixed(1)}</div>
  {/if}

  <div class="phead dim">Training cost (skill points)</div>
  <div class="train">
    <label>Cur <input type="number" min="0" bind:value={curRank} /></label>
    <label>Target <input type="number" min="0" bind:value={desRank} /></label>
    <label>Slot <input type="number" min="1" max="20" bind:value={slot} /></label>
    <label>Difficulty
      <select bind:value={tdiff}>
        {#each DIFFS as d, i (d)}<option value={i}>{d}</option>{/each}
      </select>
    </label>
  </div>
  <div class="flags row">
    <label class="f"><input type="checkbox" bind:checked={selfTrained} /> Self-trained</label>
    <label class="f"><input type="checkbox" bind:checked={selfTaught} /> Self-taught</label>
    <label class="f"><input type="checkbox" bind:checked={healing} /> Healing</label>
    <span class="spacer"></span>
    <span class="cost">{cost} SP</span>
  </div>
</Modal>

<style>
  .controls {
    gap: 14px;
    margin-bottom: 14px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
    color: var(--fg-dim);
  }
  .controls input,
  .train input {
    width: 80px;
  }
  table.rb {
    width: 100%;
    border-collapse: collapse;
    font-family: var(--mono);
    font-size: 13px;
  }
  table.rb th,
  table.rb td {
    border: 1px solid var(--border);
    padding: 5px 8px;
    text-align: center;
  }
  table.rb th {
    color: var(--fg-dim);
    font-weight: 500;
  }
  .ph {
    text-align: left !important;
    color: var(--accent);
  }
  .tier {
    margin-top: 8px;
    font-size: 12px;
  }
  .phead {
    margin: 18px 0 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .train {
    display: flex;
    gap: 12px;
    margin-bottom: 10px;
  }
  .flags {
    gap: 16px;
  }
  .f {
    flex-direction: row;
    align-items: center;
    gap: 6px;
    color: var(--fg);
    font-size: 13px;
  }
  .cost {
    font-family: var(--mono);
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
  }
</style>
