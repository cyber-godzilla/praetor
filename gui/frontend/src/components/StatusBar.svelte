<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { vitalColor } from "../lib/vitals";

  const WIDTH = 10; // characters per bar

  // Clicking any vital bar checks condition; clicking the lighting readout
  // checks the ambient light level.
  function checkCond() {
    api.send("cond");
  }
  function checkLighting() {
    api.send("lighting");
  }

  interface Bar {
    label: string;
    tip: string;
    value: number | null;
  }

  const bars = $derived<Bar[]>([
    { label: "HP", tip: "Health", value: store.health },
    { label: "FT", tip: "Fatigue", value: store.fatigue },
    { label: "EN", tip: "Encumbrance", value: store.encumbrance },
    { label: "SA", tip: "Satiation", value: store.satiation },
  ]);

  function filledCount(v: number | null): number {
    if (v == null) return 0;
    const pct = Math.max(0, Math.min(100, v));
    return Math.round((pct / 100) * WIDTH);
  }
  const block = (n: number) => "█".repeat(n);
  const dots = (n: number) => "░".repeat(n);

  // Lighting label + color from the raw SKOOT ch9 value (approx, per CLAUDE.md).
  const lighting = $derived.by(() => {
    const r = store.lightingRaw;
    if (r == null) return null;
    if (r >= 100) return { text: "Extremely Bright", color: "var(--light-blinding)" };
    if (r >= 30) return { text: "Very Bright", color: "var(--light-verybright)" };
    if (r >= 25) return { text: "Bright", color: "var(--light-bright)" };
    if (r >= 18) return { text: "Fairly Lit", color: "var(--light-fairlylit)" };
    if (r >= 12) return { text: "Somewhat Dark", color: "var(--light-somewhatdark)" };
    if (r >= 6) return { text: "Very Dark", color: "var(--light-verydark)" };
    if (r >= 1) return { text: "Extremely Dark", color: "var(--light-extremedark)" };
    return { text: "Pitch Black", color: "var(--light-pitchblack)" };
  });

  const conn = $derived(
    store.connState === "connected"
      ? { text: "● CONNECTED", cls: "ok" }
      : { text: "○ " + (store.connReason || "DISCONNECTED").toUpperCase(), cls: "bad" },
  );
</script>

<div class="statusbar">
  <div class="bars">
    {#each bars as b (b.label)}
      <button class="bar" title="{b.tip} — click to check condition" onclick={checkCond} tabindex="-1">
        <span class="lbl">{b.label}</span>
        <span class="bracket">[</span><span
          style="color:{vitalColor(b.value)}">{block(filledCount(b.value))}</span><span
          class="empty">{dots(WIDTH - filledCount(b.value))}</span><span class="bracket">]</span>
        <span class="num" style="color:{vitalColor(b.value)}">{b.value ?? "—"}</span>
      </button>
    {/each}
  </div>
  {#if lighting}
    <button class="lighting" style="color:{lighting.color}" title="Lighting — click to check" onclick={checkLighting} tabindex="-1">☀ {lighting.text}</button>
  {/if}
  <span class="spacer"></span>
  <span class="conn {conn.cls}">{conn.text}</span>
</div>

<style>
  .statusbar {
    display: flex;
    align-items: center;
    gap: 18px;
    padding: 4px 10px;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    font-size: 12px;
    white-space: nowrap;
  }
  .bars {
    display: flex;
    gap: 16px;
  }
  .bar {
    letter-spacing: -0.5px;
    font: inherit;
    color: inherit;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
  }
  /* Neutralise the global button:hover (border/bright color); use a subtle
     label tint to signal the bar is clickable. */
  .bar:hover {
    color: inherit;
  }
  .bar:hover .lbl {
    color: var(--accent);
  }
  .lbl {
    color: var(--fg-dim);
    margin-right: 4px;
  }
  .bracket {
    color: var(--fg-dim);
  }
  .empty {
    color: var(--bar-empty);
  }
  .num {
    margin-left: 4px;
  }
  .lighting {
    color: var(--fg-dim);
    font: inherit;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
  }
  .lighting:hover {
    filter: brightness(1.3);
  }
  .conn.ok {
    color: var(--green);
  }
  .conn.bad {
    color: var(--red);
  }
</style>
