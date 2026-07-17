<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  const FONT_SIZES = [11, 12, 13, 14, 16, 18, 20, 24];

  // Buffered draft: edit locally, apply on Save, discard on Cancel (like the
  // Highlights editor).
  const seed = untrack(() => store.config);
  let echoTyped = $state(seed?.UI?.EchoTyped ?? true);
  let echoScript = $state(seed?.UI?.EchoScript ?? true);
  let colorWords = $state(seed?.UI?.ColorWords ?? false);
  let hideIPs = $state(seed?.UI?.HideIPs ?? false);
  let sessionLogging = $state(seed?.Logging?.Session?.Enabled ?? false);
  let logPath = $state(seed?.Logging?.Session?.Path ?? "");
  let minimapScale = $state(seed?.UI?.MinimapScale ?? 1);
  let compassScale = $state(seed?.UI?.CompassScale ?? 1);
  let fontSize = $state(seed?.UI?.OutputFontSize || 14);
  let numpadNav = $state(seed?.UI?.NumpadNavigation ?? "numlock");

  async function save() {
    try {
      // Sequential: each setter persists the whole config, so avoid concurrent
      // file writes.
      await api.setEchoTyped(echoTyped);
      await api.setEchoScript(echoScript);
      await api.setColorWords(colorWords);
      await api.setHideIPs(hideIPs);
      await api.setSessionLogging(sessionLogging);
      await api.setLogPath(logPath);
      await api.setMinimapScale(minimapScale);
      await api.setCompassScale(compassScale);
      await api.setOutputFontSize(fontSize);
      await api.setNumpadNavigation(numpadNav);
      if (store.config) {
        Object.assign(store.config.UI, {
          EchoTyped: echoTyped,
          EchoScript: echoScript,
          ColorWords: colorWords,
          HideIPs: hideIPs,
          MinimapScale: minimapScale,
          CompassScale: compassScale,
          OutputFontSize: fontSize,
          NumpadNavigation: numpadNav,
        });
        store.config.Logging.Session.Enabled = sessionLogging;
        store.config.Logging.Session.Path = logPath;
      }
      store.addToast("Settings", "Saved");
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="Settings" back onsave={save}>
  <div class="toggles">
    <label class="t"><span>Echo typed commands</span><input type="checkbox" bind:checked={echoTyped} /></label>
    <label class="t"><span>Echo script commands</span><input type="checkbox" bind:checked={echoScript} /></label>
    <label class="t"><span>Color words</span><input type="checkbox" bind:checked={colorWords} /></label>
    <label class="t"><span>Hide IP addresses</span><input type="checkbox" bind:checked={hideIPs} /></label>
    <label class="t"><span>Session transcript logging</span><input type="checkbox" bind:checked={sessionLogging} /></label>

    <div class="field">
      <span>Minimap scale</span>
      <input type="number" min="0.2" max="3" step="0.1" bind:value={minimapScale} />
    </div>
    <div class="field">
      <span>Compass scale</span>
      <input type="number" min="0.5" max="3" step="0.1" bind:value={compassScale} />
    </div>
    <div class="field">
      <span>Output text size</span>
      <div class="sizes">
        {#each FONT_SIZES as sz (sz)}
          <button class="sz" class:active={fontSize === sz} onclick={() => (fontSize = sz)}>{sz}</button>
        {/each}
      </div>
    </div>
    <div class="field">
      <span>Numpad navigation</span>
      <select bind:value={numpadNav}>
        <option value="numlock">Only when NumLock is off</option>
        <option value="always">Always</option>
        <option value="off">Never</option>
      </select>
    </div>
    <div class="field">
      <span>Log path (blank = default)</span>
      <input type="text" bind:value={logPath} />
    </div>
  </div>

</Modal>

<style>
  .toggles {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .t {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 9px 4px;
    font-size: 14px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 10px 4px;
    font-size: 13px;
    color: var(--fg-dim);
  }
  .sizes {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
  }
  .sz {
    min-width: 40px;
    padding: 5px 8px;
    font-size: 12px;
  }
  .sz.active {
    background: var(--accent-dim);
    border-color: var(--accent);
    color: var(--fg-bright);
  }
</style>
