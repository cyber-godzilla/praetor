<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  const DESKTOP_FONT_SIZES = [11, 12, 13, 14, 16, 18, 20, 24];
  const MOBILE_FONT_SIZES = [6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 18, 20, 24];

  // Buffered draft: edit locally, apply on Save, discard on Cancel (like the
  // Highlights editor).
  const seed = untrack(() => store.config);
  let echoTyped = $state(seed?.UI?.EchoTyped ?? true);
  let echoScript = $state(seed?.UI?.EchoScript ?? true);
  let colorWords = $state(seed?.UI?.ColorWords ?? false);
  let hideIPs = $state(seed?.UI?.HideIPs ?? false);
  let inputSpellcheck = $state(seed?.UI?.InputSpellcheck ?? true);
  let updateCheck = $state(seed?.Updates?.Check ?? true);
  let mobileShowToolbar = $state(seed?.UI?.MobileShowToolbar ?? true);
  let mobileShowTabBar = $state(seed?.UI?.MobileShowTabBar ?? true);
  let mobileHideNavigationOnInput = $state(seed?.UI?.MobileHideNavigationOnInput ?? false);
  let mobileLowercaseFirstLetter = $state(seed?.UI?.MobileLowercaseFirstLetter ?? false);
  let retainAppLogs = $state(seed?.Logging?.App?.Retain ?? false);
  let sessionLogging = $state(seed?.Logging?.Session?.Enabled ?? false);
  let logPath = $state(seed?.Logging?.Session?.Path ?? "");
  let minimapScale = $state(seed?.UI?.MinimapScale ?? 1);
  let compassScale = $state(seed?.UI?.CompassScale ?? 1);
  let fontSize = $state(seed?.UI?.OutputFontSize || 14);
  let mobileFontSize = $state(seed?.UI?.MobileOutputFontSize || seed?.UI?.OutputFontSize || 14);
  let numpadNav = $state(seed?.UI?.NumpadNavigation ?? "numlock");

  async function save() {
    try {
      // Sequential: each setter persists the whole config, so avoid concurrent
      // file writes.
      await api.setEchoTyped(echoTyped);
      await api.setEchoScript(echoScript);
      await api.setColorWords(colorWords);
      await api.setHideIPs(hideIPs);
      await api.setInputSpellcheck(inputSpellcheck);
      await api.setUpdateCheck(updateCheck);
      if (api.inWeb()) {
        await api.setMobileShowToolbar(mobileShowToolbar);
        await api.setMobileShowTabBar(mobileShowTabBar);
        await api.setMobileHideNavigationOnInput(mobileHideNavigationOnInput);
        await api.setMobileLowercaseFirstLetter(mobileLowercaseFirstLetter);
        await api.setMobileOutputFontSize(mobileFontSize);
      }
      await api.setLogPath(logPath);
      await api.setRetainAppLogs(retainAppLogs);
      await api.setSessionLogging(sessionLogging);
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
          InputSpellcheck: inputSpellcheck,
          MobileShowToolbar: mobileShowToolbar,
          MobileShowTabBar: mobileShowTabBar,
          MobileHideNavigationOnInput: mobileHideNavigationOnInput,
          MobileLowercaseFirstLetter: mobileLowercaseFirstLetter,
          MobileOutputFontSize: mobileFontSize,
          MinimapScale: minimapScale,
          CompassScale: compassScale,
          OutputFontSize: fontSize,
          NumpadNavigation: numpadNav,
        });
        store.config.Updates = { Check: updateCheck };
        store.config.Logging.App.Retain = retainAppLogs;
        store.config.Logging.Session.Enabled = sessionLogging;
        store.config.Logging.Session.Path = logPath;
      }
      store.addToast("Settings", "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title="Settings" back onsave={save}>
  <div class="toggles">
    <label class="t"><span>Echo typed commands</span><input type="checkbox" bind:checked={echoTyped} /></label>
    <label class="t"><span>Echo script commands</span><input type="checkbox" bind:checked={echoScript} /></label>
    <label class="t"><span>Color words</span><input type="checkbox" bind:checked={colorWords} /></label>
    <label class="t"><span>Hide IP addresses</span><input type="checkbox" bind:checked={hideIPs} /></label>
    <label class="t"><span>Input spellcheck</span><input type="checkbox" bind:checked={inputSpellcheck} /></label>
    <label class="t"><span>Check for updates on startup</span><input type="checkbox" bind:checked={updateCheck} /></label>
    <label class="t"><span>Session transcript logging</span><input type="checkbox" bind:checked={sessionLogging} /></label>
    <label class="t"><span>Retain application logs (applies next launch)</span><input type="checkbox" bind:checked={retainAppLogs} /></label>

    <div class="field">
      <span>Minimap scale</span>
      <input type="number" min="0.2" max="3" step="0.1" bind:value={minimapScale} />
    </div>
    <div class="field">
      <span>Compass scale</span>
      <input type="number" min="0.5" max="3" step="0.1" bind:value={compassScale} />
    </div>
    <div class="field">
      <span>{api.inWeb() ? "Desktop output text size" : "Output text size"}</span>
      <div class="sizes">
        {#each DESKTOP_FONT_SIZES as sz (sz)}
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
      <span>{api.inWeb() ? "Log path on server host (blank = default)" : "Log path (blank = default)"}</span>
      <input type="text" bind:value={logPath} />
    </div>
    {#if api.inWeb()}
      <fieldset class="mobile-settings">
        <legend>Mobile web</legend>
        <label class="t"><span>Show Actions / Modes / Menu row on mobile</span><input type="checkbox" bind:checked={mobileShowToolbar} /></label>
        <label class="t"><span>Show tab selector on mobile</span><input type="checkbox" bind:checked={mobileShowTabBar} /></label>
        <label class="t"><span>Hide map and compass while command input is active</span><input type="checkbox" bind:checked={mobileHideNavigationOnInput} /></label>
        <label class="t"><span>Lowercase the first command letter on mobile</span><input type="checkbox" bind:checked={mobileLowercaseFirstLetter} /></label>
        <div class="field mobile-font-size">
          <span>Mobile output text size</span>
          <div class="sizes">
            {#each MOBILE_FONT_SIZES as sz (sz)}
              <button class="sz" class:active={mobileFontSize === sz} onclick={() => (mobileFontSize = sz)}>{sz}</button>
            {/each}
          </div>
        </div>
      </fieldset>
    {/if}
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
    gap: 12px;
    padding: 9px 4px;
    font-size: 14px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
  }
  .t input {
    flex: 0 0 auto;
  }
  .mobile-settings {
    margin: 8px 0 0;
    min-width: 0;
    padding: 0;
    border: 0;
  }
  .mobile-settings legend {
    padding: 0 4px 4px;
    color: var(--fg-dim);
    font-size: 13px;
    font-weight: 600;
  }
  .mobile-font-size {
    padding-bottom: 2px;
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
