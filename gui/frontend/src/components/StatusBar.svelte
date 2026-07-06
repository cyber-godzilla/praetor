<script lang="ts">
  import { store } from "../lib/store.svelte";

  interface Bar {
    label: string;
    value: number | null;
    color: string;
  }

  const bars = $derived<Bar[]>([
    { label: "HP", value: store.health, color: "var(--green)" },
    { label: "FAT", value: store.fatigue, color: "var(--blue)" },
    { label: "ENC", value: store.encumbrance, color: "var(--accent)" },
    { label: "SAT", value: store.satiation, color: "#c46cc4" },
  ]);

  function pct(v: number | null): number {
    if (v == null) return 0;
    return Math.max(0, Math.min(100, v));
  }

  // Lighting label from the raw SKOOT ch9 value (approximate, per CLAUDE.md).
  const lightingLabel = $derived.by(() => {
    const r = store.lightingRaw;
    if (r == null) return "";
    if (r >= 100) return "Extremely Bright";
    if (r >= 30) return "Very Bright";
    if (r >= 25) return "Bright";
    if (r >= 18) return "Fairly Lit";
    if (r >= 12) return "Somewhat Dark";
    if (r >= 6) return "Very Dark";
    if (r >= 1) return "Extremely Dark";
    return "Pitch Black";
  });

  const connLabel = $derived.by(() => {
    switch (store.connState) {
      case "connected":
        return { text: "connected", cls: "ok" };
      case "reconnecting":
        return { text: `reconnecting (#${store.reconnectAttempt})`, cls: "warn" };
      default:
        return { text: store.connReason || "disconnected", cls: "bad" };
    }
  });
</script>

<div class="statusbar">
  <div class="brand">praetor</div>
  <div class="bars">
    {#each bars as b (b.label)}
      <div class="bar" title={b.label}>
        <span class="lbl">{b.label}</span>
        <div class="track">
          <div class="fill" style="width:{pct(b.value)}%;background:{b.color}"></div>
        </div>
        <span class="num">{b.value ?? "–"}</span>
      </div>
    {/each}
  </div>
  {#if lightingLabel}
    <div class="lighting" title="Lighting">☀ {lightingLabel}</div>
  {/if}
  <div class="spacer"></div>
  <div class="conn {connLabel.cls}">● {connLabel.text}</div>
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
  .brand {
    color: var(--accent);
    font-weight: 700;
    letter-spacing: 1px;
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
    background: var(--bg-input);
    border-radius: 4px;
    overflow: hidden;
  }
  .fill {
    height: 100%;
    transition: width 0.25s ease;
  }
  .num {
    width: 26px;
    text-align: right;
    font-family: var(--mono);
  }
  .lighting {
    color: var(--fg-dim);
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
