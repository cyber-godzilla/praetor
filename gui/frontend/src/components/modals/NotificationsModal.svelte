<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { DesktopNotificationsConfig, NotifyPatternConfig } from "../../lib/types";

  const src = store.config?.Notifications?.Desktop;
  let cfg = $state<DesktopNotificationsConfig>({
    HealthBelow: { ...(src?.HealthBelow ?? { Enabled: false, Threshold: 25 }) },
    FatigueBelow: { ...(src?.FatigueBelow ?? { Enabled: false, Threshold: 10 }) },
    Patterns: (src?.Patterns ?? []).map((p) => ({ ...p })),
  });

  function addPattern() {
    const p: NotifyPatternConfig = { Pattern: "", Title: "", Message: "", Enabled: true };
    cfg.Patterns = [...(cfg.Patterns ?? []), p];
  }
  function removePattern(i: number) {
    cfg.Patterns = (cfg.Patterns ?? []).filter((_, idx) => idx !== i);
  }

  async function save() {
    try {
      await api.setNotifications(cfg);
      store.config!.Notifications.Desktop = cfg;
      store.addToast("Notifications", "Saved");
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="Desktop Notifications" wide back>
  <div class="section">
    <label class="chk"><input type="checkbox" bind:checked={cfg.HealthBelow.Enabled} /> Notify when health below</label>
    <input class="num" type="number" min="0" max="100" bind:value={cfg.HealthBelow.Threshold} />
  </div>
  <div class="section">
    <label class="chk"><input type="checkbox" bind:checked={cfg.FatigueBelow.Enabled} /> Notify when fatigue below</label>
    <input class="num" type="number" min="0" max="100" bind:value={cfg.FatigueBelow.Threshold} />
  </div>

  <div class="phead dim">Text patterns</div>
  {#each cfg.Patterns ?? [] as p, i (i)}
    <div class="pat">
      <input type="checkbox" bind:checked={p.Enabled} title="Enabled" />
      <input type="text" bind:value={p.Pattern} placeholder="match text" />
      <input type="text" bind:value={p.Title} placeholder="title (optional)" />
      <button class="danger sm" onclick={() => removePattern(i)}>✕</button>
    </div>
  {/each}
  <button class="sm" onclick={addPattern}>+ Add pattern</button>

  {#snippet footer()}
    <button onclick={() => (store.openModal = null)}>Cancel</button>
    <button class="primary" onclick={save}>Save</button>
  {/snippet}
</Modal>

<style>
  .section {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
  }
  .num {
    width: 70px;
  }
  .phead {
    margin: 14px 0 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .pat {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 6px;
  }
  .pat input[type="text"] {
    flex: 1;
  }
  .sm {
    padding: 4px 10px;
    font-size: 12px;
  }
</style>
