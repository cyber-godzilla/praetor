<script lang="ts">
  import { store } from "../lib/store.svelte";

  interface Bar {
    label: string;
    value: number | null;
  }

  const bars = $derived<Bar[]>([
    { label: "HP", value: store.health },
    { label: "FT", value: store.fatigue },
    { label: "EN", value: store.encumbrance },
    { label: "SAT", value: store.satiation },
  ]);

  function pct(v: number | null): number {
    if (v == null) return 0;
    return Math.max(0, Math.min(100, v));
  }

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

  const connLabel = $derived.by(() => {
    switch (store.connState) {
      case "connected":
        return { text: "Connected ●", cls: "ok" };
      case "reconnecting":
        return { text: `Reconnecting (${store.reconnectAttempt}) ◌`, cls: "warn" };
      default:
        return { text: (store.connReason || "Disconnected") + " ○", cls: "bad" };
    }
  });
</script>

<div class="statusbar">
  <div class="bars">
    {#each bars as b (b.label)}
      <div class="bar" title={b.label}>
        <span class="lbl">{b.label}</span>
        <div class="track">
          <div class="fill" style="width:{pct(b.value)}%;background:{vitalColor(b.value)}"></div>
        </div>
        <span class="num" style="color:{vitalColor(b.value)}">{b.value ?? "–"}</span>
      </div>
    {/each}
  </div>
  {#if lighting}
    <div class="lighting" style="color:{lighting.color}" title="Lighting">☀ {lighting.text}</div>
  {/if}
  <div class="spacer"></div>
  <div class="conn {connLabel.cls}">{connLabel.text}</div>
</div>

<style>
  .statusbar {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 6px 12px;
    background: var(--bg-elevated);
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .bars {
    display: flex;
    gap: 14px;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .lbl {
    color: var(--fg-dim);
    width: 26px;
  }
  .track {
    width: 80px;
    height: 8px;
    background: var(--bar-empty);
    border-radius: 4px;
    overflow: hidden;
  }
  .fill {
    height: 100%;
    transition: width 0.25s ease, background 0.25s ease;
  }
  .num {
    width: 26px;
    text-align: right;
    font-family: var(--mono);
  }
  .lighting {
    font-family: var(--mono);
  }
  .conn.ok {
    color: var(--green);
  }
  .conn.warn {
    color: var(--accent);
  }
  .conn.bad {
    color: var(--red);
  }
</style>
