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

  async function enableBrowserNotifications() {
    const result = await api.requestNotificationPermission();
    if (result === "granted") {
      store.addToast("Browser notifications", "Enabled for this browser.");
    } else if (result === "unsupported") {
      store.addToast("Browser notifications unavailable", "Use HTTPS or allow in-app notifications only.");
    } else {
      store.addToast("Browser notifications", "Permission was not granted.");
    }
  }

  async function save() {
    try {
      await api.setNotifications(cfg);
      store.config!.Notifications.Desktop = cfg;
      store.addToast("Notifications", "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title={api.inWeb() ? "Notifications" : "Desktop Notifications"} wide back onsave={save}>
  {#if api.inWeb()}
    <div class="browser-notify">
      <span class="dim">Matches always appear in every client as in-app alerts. Browser-native alerts are a permission local to this device.</span>
      <button onclick={enableBrowserNotifications}>Enable on this browser</button>
    </div>
  {/if}
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

</Modal>

<style>
  .section {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  .browser-notify {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 0 10px;
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .browser-notify span {
    flex: 1;
  }
  .browser-notify button {
    flex: 0 0 auto;
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
  @media (max-width: 599px) {
    .browser-notify,
    .section {
      align-items: stretch;
      flex-direction: column;
    }
    .pat {
      display: grid;
      grid-template-columns: 20px minmax(0, 1fr) 40px;
    }
    .pat input:nth-of-type(3) {
      grid-column: 2;
    }
  }
</style>
