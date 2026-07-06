<script lang="ts">
  import { store } from "../lib/store.svelte";

  const WIDTH = 10; // characters per bar

  interface Bar {
    label: string;
    value: number | null;
  }

  const bars = $derived<Bar[]>([
    { label: "HP", value: store.health },
    { label: "FT", value: store.fatigue },
    { label: "EN", value: store.encumbrance },
    { label: "SA", value: store.satiation },
  ]);

  function filledCount(v: number | null): number {
    if (v == null) return 0;
    const pct = Math.max(0, Math.min(100, v));
    return Math.round((pct / 100) * WIDTH);
  }
  const block = (n: number) => "█".repeat(n);
  const dots = (n: number) => "░".repeat(n);

  // vitalColor mirrors internal/ui/statusbar.go: >50 green, >25 orange, else red.
  function vitalColor(v: number | null): string {
    if (v == null) return "var(--fg-dim)";
    if (v > 50) return "var(--green)";
    if (v > 25) return "var(--accent)";
    return "var(--red)";
  }

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
      <span class="bar" title={b.label}>
        <span class="lbl">{b.label}</span>
        <span class="bracket">[</span><span
          style="color:{vitalColor(b.value)}">{block(filledCount(b.value))}</span><span
          class="empty">{dots(WIDTH - filledCount(b.value))}</span><span class="bracket">]</span>
        <span class="num" style="color:{vitalColor(b.value)}">{b.value ?? "—"}</span>
      </span>
    {/each}
  </div>
  {#if lighting}
    <span class="lighting" style="color:{lighting.color}" title="Lighting">☀ {lighting.text}</span>
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
  }
  .conn.ok {
    color: var(--green);
  }
  .conn.bad {
    color: var(--red);
  }
</style>
