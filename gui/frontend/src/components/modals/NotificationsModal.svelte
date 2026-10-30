<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { DesktopNotificationsConfig, DiscordNotificationsConfig, NotifyPatternConfig, ThresholdConfig } from "../../lib/types";
  import ExpansionPreview from "../ExpansionPreview.svelte";

  type Delivery = "desktop" | "discord" | "both" | "neither";
  type When = "always" | "unfocused";

  const src = store.config?.Notifications?.Desktop;
  const discordSrc = store.config?.Notifications?.Discord;

  function threshold(source: ThresholdConfig | undefined, fallback: number, discordEnabled: boolean): ThresholdConfig {
    return {
      Enabled: source?.Enabled ?? false,
      Threshold: source?.Threshold ?? fallback,
      When: source?.When ?? "always",
      Sound: source?.Sound ?? src?.Sound ?? false,
      Delivery: source?.Delivery ?? (discordEnabled ? "both" : "desktop"),
    };
  }

  function pattern(source: NotifyPatternConfig): NotifyPatternConfig {
    return {
      ...source,
      When: source.When ?? "always",
      Sound: source.Sound ?? src?.Sound ?? false,
      Delivery: source.Delivery ?? (discordSrc?.Patterns ? "both" : "desktop"),
    };
  }

  let cfg = $state<DesktopNotificationsConfig>({
    AllowScriptNotifications: src?.AllowScriptNotifications ?? false,
    Sound: src?.Sound ?? false,
    ScriptSound: src?.ScriptSound ?? src?.Sound ?? false,
    HealthBelow: threshold(src?.HealthBelow, 25, discordSrc?.Health ?? false),
    FatigueBelow: threshold(src?.FatigueBelow, 10, discordSrc?.Fatigue ?? false),
    Patterns: (src?.Patterns ?? []).map(pattern),
  });
  let discord = $state<DiscordNotificationsConfig>({
    Enabled: discordSrc?.Enabled ?? false,
    WebhookURL: discordSrc?.WebhookURL ?? "",
    Health: discordSrc?.Health ?? false,
    Fatigue: discordSrc?.Fatigue ?? false,
    Patterns: discordSrc?.Patterns ?? false,
    Scripts: discordSrc?.Scripts ?? false,
  });

  function addPattern() {
    cfg.Patterns = [...(cfg.Patterns ?? []), {
      Pattern: "", Title: "", Message: "", Enabled: true,
      When: "always", Sound: false, Delivery: "desktop",
    }];
  }

  function removePattern(index: number) {
    cfg.Patterns = (cfg.Patterns ?? []).filter((_, candidate) => candidate !== index);
  }

  function includesDiscord(delivery: Delivery | undefined) {
    return delivery === "discord" || delivery === "both";
  }

  function syncLegacyFields() {
    discord.Health = includesDiscord(cfg.HealthBelow.Delivery);
    discord.Fatigue = includesDiscord(cfg.FatigueBelow.Delivery);
    discord.Patterns = (cfg.Patterns ?? []).some((rule) => includesDiscord(rule.Delivery));
    cfg.Sound = !!cfg.ScriptSound || !!cfg.HealthBelow.Sound || !!cfg.FatigueBelow.Sound ||
      (cfg.Patterns ?? []).some((rule) => !!rule.Sound);
  }

  async function enableBrowserNotifications() {
    const result = await api.requestNotificationPermission();
    if (result === "granted") store.addToast("Browser notifications", "Enabled for this browser.");
    else if (result === "unsupported") store.addToast("Browser notifications unavailable", "Use HTTPS or allow in-app notifications only.");
    else store.addToast("Browser notifications", "Permission was not granted.");
  }

  async function save() {
    try {
      syncLegacyFields();
      await api.setNotifications({ Desktop: cfg, Discord: discord });
      store.config!.Notifications.Desktop = cfg;
      store.config!.Notifications.Discord = discord;
      store.addToast("Notifications", "Saved");
      store.openModal = null;
    } catch (error) {
      store.addToast("Save failed", String(error));
    }
  }

  async function testDiscord() {
    try {
      syncLegacyFields();
      await api.setNotifications({ Desktop: cfg, Discord: discord });
      store.config!.Notifications.Desktop = cfg;
      store.config!.Notifications.Discord = discord;
      await api.testDiscordNotification();
      store.addToast("Discord notifications", "Test notification sent.");
    } catch (error) {
      store.addToast("Discord test failed", String(error));
    }
  }
</script>

{#snippet ruleOptions(rule: { When?: When; Sound?: boolean; Delivery?: Delivery }, label: string)}
  <div class="rule-options" aria-label={`${label} notification options`}>
    <label><span>When</span><select bind:value={rule.When}><option value="always">Focused or unfocused</option><option value="unfocused">Only while unfocused</option></select></label>
    <label class="sound"><input type="checkbox" bind:checked={rule.Sound} /> Sound</label>
    <label><span>Send to</span><select bind:value={rule.Delivery}><option value="desktop">Desktop</option><option value="discord">Discord</option><option value="both">Both</option><option value="neither">Neither</option></select></label>
  </div>
{/snippet}

<Modal title="Notifications" wide back onsave={save}>
  {#if api.inWeb()}
    <div class="browser-notify">
      <span class="dim">Browser alerts and “only while unfocused” are evaluated separately on each device.</span>
      <button onclick={enableBrowserNotifications}>Enable on this browser</button>
    </div>
  {/if}

  <div class="phead">Built-in alerts</div>
  <section class="rule-card">
    <header><label class="chk"><input type="checkbox" bind:checked={cfg.HealthBelow.Enabled} /><strong>Health</strong></label><label class="threshold"><span>Below</span><input type="number" min="0" max="100" bind:value={cfg.HealthBelow.Threshold} /><span>%</span></label></header>
    {@render ruleOptions(cfg.HealthBelow, "Health")}
  </section>
  <section class="rule-card">
    <header><label class="chk"><input type="checkbox" bind:checked={cfg.FatigueBelow.Enabled} /><strong>Fatigue</strong></label><label class="threshold"><span>Below</span><input type="number" min="0" max="100" bind:value={cfg.FatigueBelow.Threshold} /><span>%</span></label></header>
    {@render ruleOptions(cfg.FatigueBelow, "Fatigue")}
  </section>

  <div class="phead">String matches</div>
  <p class="section-help">Each text rule controls its own focus behavior, sound, and destination.</p>
  <div class="patterns">
    {#each cfg.Patterns ?? [] as rule, index (index)}
      <section class="rule-card pattern-card">
        <header><label class="chk"><input type="checkbox" bind:checked={rule.Enabled} /><strong>String {index + 1}</strong></label><button class="danger sm" onclick={() => removePattern(index)} aria-label={`Delete string rule ${index + 1}`}>✕</button></header>
        <div class="pattern-fields">
          <label><span>Match</span><span class="field"><input type="text" bind:value={rule.Pattern} placeholder={'text, * ? or ${name}'} /><ExpansionPreview value={rule.Pattern} match /></span></label>
          <label><span>Title</span><span class="field"><input type="text" bind:value={rule.Title} placeholder="Alert" /><ExpansionPreview value={rule.Title} /></span></label>
          <label><span>Message</span><span class="field"><input type="text" bind:value={rule.Message} placeholder="Matched game text" /><ExpansionPreview value={rule.Message} /></span></label>
        </div>
        {@render ruleOptions(rule, `String ${index + 1}`)}
      </section>
    {/each}
  </div>
  <button class="add" onclick={addPattern}>+ Add string match</button>

  <div class="phead">Lua script alerts</div>
  <section class="rule-card script-card">
    <label class="chk"><input type="checkbox" bind:checked={cfg.AllowScriptNotifications} /> Enable script notifications</label>
    <label class="chk"><input type="checkbox" bind:checked={cfg.ScriptSound} /> Sound</label>
    <label class="chk"><input type="checkbox" bind:checked={discord.Scripts} /> Also send to Discord</label>
  </section>

  <div class="phead">Discord webhook</div>
  <div class="discord">
    <label class="chk"><input type="checkbox" bind:checked={discord.Enabled} /> Enable Discord delivery</label>
    <label class="webhook"><span>Webhook URL</span><input type="password" bind:value={discord.WebhookURL} autocomplete="off" placeholder="https://discord.com/api/webhooks/…" /></label>
    <div class="discord-actions"><span class="dim">Rules set to Discord or Both use this webhook. The credential is never written to logs.</span><button class="sm" onclick={testDiscord}>Save and send test</button></div>
  </div>
</Modal>

<style>
  .browser-notify { display: flex; align-items: center; gap: 12px; padding: 0 0 10px; border-bottom: 1px solid var(--border); font-size: 11px; }
  .browser-notify span { flex: 1; }
  .phead { margin: 16px 0 7px; color: var(--accent); font-size: 11px; font-weight: 700; letter-spacing: 1px; text-transform: uppercase; }
  .section-help { margin: -2px 0 8px; color: var(--fg-dim); font-size: 11px; }
  .rule-card { display: grid; gap: 9px; margin-bottom: 8px; padding: 10px; border: 1px solid var(--border); background: color-mix(in srgb, var(--bg-input) 55%, transparent); }
  .rule-card header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .chk, .threshold, .sound { display: flex; align-items: center; gap: 7px; }
  .chk { font-size: 13px; }
  .threshold { color: var(--fg-dim); font-size: 11px; }
  .threshold input { width: 64px; }
  .rule-options { display: grid; grid-template-columns: minmax(170px, 1fr) auto minmax(130px, .7fr); align-items: end; gap: 10px; }
  .rule-options > label:not(.sound), .webhook { display: grid; gap: 4px; color: var(--fg-dim); font-size: 10px; text-transform: uppercase; }
  .sound { min-height: 30px; font-size: 12px; }
  .pattern-fields { display: grid; grid-template-columns: 1.2fr .8fr 1fr; gap: 8px; }
  .pattern-fields > label { min-width: 0; display: grid; gap: 4px; color: var(--fg-dim); font-size: 10px; text-transform: uppercase; }
  .field, .field input { min-width: 0; width: 100%; box-sizing: border-box; }
  .sm { padding: 4px 10px; font-size: 12px; }
  .add { width: 100%; border-style: dashed; }
  .script-card { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .discord { display: grid; gap: 10px; }
  .webhook input { width: 100%; box-sizing: border-box; }
  .discord-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 11px; }
  @media (max-width: 700px) {
    .browser-notify, .discord-actions { align-items: stretch; flex-direction: column; }
    .rule-options, .pattern-fields, .script-card { grid-template-columns: 1fr; }
    .rule-options { align-items: stretch; }
  }
</style>
