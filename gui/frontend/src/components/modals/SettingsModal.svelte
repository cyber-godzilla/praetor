<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  type SettingsSection = "all" | "display" | "input" | "logs" | "application";
  let { section = "all" }: { section?: SettingsSection } = $props();

  const title = $derived(
    section === "display" ? "Text & Display Settings"
      : section === "input" ? "Input Behavior"
      : section === "logs" ? "Logging"
      : section === "application" ? "Application Settings"
      : "Settings",
  );

  const DESKTOP_FONT_SIZES = [11, 12, 13, 14, 16, 18, 20, 24];
  const MOBILE_FONT_SIZES = [6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 18, 20, 24];
  const WOUND_FAMILIES = [
    { value: "red", label: "Red" },
    { value: "purple", label: "Purple" },
    { value: "blue", label: "Blue" },
    { value: "orange", label: "Orange" },
  ];

  // Buffered draft: edit locally, apply on Save, discard on Cancel (like the
  // Highlights editor).
  const seed = untrack(() => store.config);
  let echoTyped = $state(seed?.UI?.EchoTyped ?? true);
  let echoScript = $state(seed?.UI?.EchoScript ?? true);
  let colorWords = $state(seed?.UI?.ColorWords ?? false);
  let hideIPs = $state(seed?.UI?.HideIPs ?? false);
  let inputSpellcheck = $state(seed?.UI?.InputSpellcheck ?? true);
  let keepInputOnSend = $state(seed?.UI?.KeepInputOnSend ?? false);
  let semicolonDelayMS = $state(seed?.Commands?.SemicolonDelayMS ?? 900);
  let unbusyDelayMS = $state(seed?.Commands?.UnbusyDelayMS ?? 100);
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
  let highlightWounds = $state(seed?.WoundHighlights?.Enabled ?? false);
  let woundCuts = $state(seed?.WoundHighlights?.Cuts ?? "red");
  let woundBruises = $state(seed?.WoundHighlights?.Bruises ?? "purple");
  let woundPunctures = $state(seed?.WoundHighlights?.Punctures ?? "blue");
  let woundBurns = $state(seed?.WoundHighlights?.Burns ?? "orange");

  async function save() {
    try {
      // Sequential: each setter persists the whole config, so avoid concurrent
      // file writes.
      if (section === "all" || section === "input") {
        await api.setEchoTyped(echoTyped);
        await api.setEchoScript(echoScript);
        await api.setInputSpellcheck(inputSpellcheck);
        await api.setKeepInputOnSend(keepInputOnSend);
        await api.setSemicolonDelay(semicolonDelayMS);
        await api.setUnbusyDelay(unbusyDelayMS);
        await api.setNumpadNavigation(numpadNav);
      }
      if (section === "all" || section === "display") {
        await api.setColorWords(colorWords);
        if (api.inWeb()) {
          await api.setMobileShowToolbar(mobileShowToolbar);
          await api.setMobileShowTabBar(mobileShowTabBar);
          await api.setMobileHideNavigationOnInput(mobileHideNavigationOnInput);
          await api.setMobileLowercaseFirstLetter(mobileLowercaseFirstLetter);
          await api.setMobileOutputFontSize(mobileFontSize);
        }
        await api.setMinimapScale(minimapScale);
        await api.setCompassScale(compassScale);
        await api.setOutputFontSize(fontSize);
        await api.setWoundHighlights({
          Enabled: highlightWounds,
          Cuts: woundCuts,
          Bruises: woundBruises,
          Punctures: woundPunctures,
          Burns: woundBurns,
        });
      }
      if (section === "all" || section === "logs") {
        await api.setLogPath(logPath);
        await api.setRetainAppLogs(retainAppLogs);
        await api.setSessionLogging(sessionLogging);
      }
      if (section === "all" || section === "application") {
        await api.setHideIPs(hideIPs);
        await api.setUpdateCheck(updateCheck);
      }
      if (store.config) {
        if (section === "all" || section === "input") {
          Object.assign(store.config.UI, {
            EchoTyped: echoTyped,
            EchoScript: echoScript,
            InputSpellcheck: inputSpellcheck,
            KeepInputOnSend: keepInputOnSend,
            NumpadNavigation: numpadNav,
          });
          store.config.Commands.SemicolonDelayMS = semicolonDelayMS;
          store.config.Commands.UnbusyDelayMS = unbusyDelayMS;
        }
        if (section === "all" || section === "display") {
          Object.assign(store.config.UI, {
            ColorWords: colorWords,
            MobileShowToolbar: mobileShowToolbar,
            MobileShowTabBar: mobileShowTabBar,
            MobileHideNavigationOnInput: mobileHideNavigationOnInput,
            MobileLowercaseFirstLetter: mobileLowercaseFirstLetter,
            MobileOutputFontSize: mobileFontSize,
            MinimapScale: minimapScale,
            CompassScale: compassScale,
            OutputFontSize: fontSize,
          });
          store.config.WoundHighlights = {
            Enabled: highlightWounds,
            Cuts: woundCuts,
            Bruises: woundBruises,
            Punctures: woundPunctures,
            Burns: woundBurns,
          };
        }
        if (section === "all" || section === "logs") {
          store.config.Logging.App.Retain = retainAppLogs;
          store.config.Logging.Session.Enabled = sessionLogging;
          store.config.Logging.Session.Path = logPath;
        }
        if (section === "all" || section === "application") {
          store.config.UI.HideIPs = hideIPs;
          store.config.Updates = { Check: updateCheck };
        }
      }
      store.addToast("Settings", "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal {title} back onsave={save}>
  <div class="toggles">
    {#if section === "all" || section === "input"}
      <label class="t"><span>Echo typed commands</span><input type="checkbox" bind:checked={echoTyped} /></label>
      <label class="t"><span>Echo script commands</span><input type="checkbox" bind:checked={echoScript} /></label>
      <label class="t"><span>Input spellcheck</span><input type="checkbox" bind:checked={inputSpellcheck} /></label>
      <label class="t"><span>Retain Input After Send</span><input type="checkbox" bind:checked={keepInputOnSend} /></label>
      <div class="field">
        <span>;; chain delay (ms)</span>
        <input aria-label="Command chain delay" type="number" min="100" max="10000" step="50" bind:value={semicolonDelayMS} />
      </div>
      <div class="field">
        <span>&amp;&amp; response delay (ms)</span>
        <input aria-label="Unbusy response delay" type="number" min="0" max="10000" step="25" bind:value={unbusyDelayMS} />
      </div>
      <div class="field">
        <span>Numpad navigation</span>
        <select bind:value={numpadNav}>
          <option value="numlock">Only when NumLock is off</option>
          <option value="always">Always</option>
          <option value="off">Never</option>
        </select>
      </div>
    {/if}

    {#if section === "all" || section === "display"}
      <fieldset class="text-color-settings">
        <legend>Text &amp; color</legend>
        <label class="t"><span>Colorwords</span><input type="checkbox" bind:checked={colorWords} /></label>
        <p>Colorwords recognizes color names in game output and renders those words using Praetor's game-matched color palette.</p>
        <div class="related-settings">
          <button onclick={() => (store.openModal = "highlights")}><span><strong>Highlights</strong><small>Character-specific pattern colors</small></span><b>›</b></button>
          <button onclick={() => (store.openModal = "speech-colors")}><span><strong>Speech colors</strong><small>Colorword styles for speakers</small></span><b>›</b></button>
        </div>
      </fieldset>
      <fieldset class="wound-settings">
        <legend>Wound highlighting</legend>
        <label class="t"><span>Highlight wounds</span><input type="checkbox" bind:checked={highlightWounds} /></label>
        <div class="wound-grid" class:disabled={!highlightWounds}>
          <label><span>Cuts</span><select bind:value={woundCuts} disabled={!highlightWounds}>{#each WOUND_FAMILIES as family}<option value={family.value}>{family.label}</option>{/each}</select></label>
          <label><span>Bruises / fractures</span><select bind:value={woundBruises} disabled={!highlightWounds}>{#each WOUND_FAMILIES as family}<option value={family.value}>{family.label}</option>{/each}</select></label>
          <label><span>Punctures / wounds</span><select bind:value={woundPunctures} disabled={!highlightWounds}>{#each WOUND_FAMILIES as family}<option value={family.value}>{family.label}</option>{/each}</select></label>
          <label><span>Burns</span><select bind:value={woundBurns} disabled={!highlightWounds}>{#each WOUND_FAMILIES as family}<option value={family.value}>{family.label}</option>{/each}</select></label>
        </div>
        <p>Severity automatically selects a stronger shade; levels 4–5 are bold.</p>
      </fieldset>
      <div class="field">
        <span>Minimap Scale (Zoom)</span>
        <input aria-label="Minimap Scale (Zoom)" type="number" min="0.2" max="3" step="0.1" bind:value={minimapScale} />
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
    {/if}

    {#if section === "all" || section === "logs"}
      <label class="t"><span>Session transcript logging</span><input type="checkbox" bind:checked={sessionLogging} /></label>
      <label class="t"><span>Retain application logs (applies next launch)</span><input type="checkbox" bind:checked={retainAppLogs} /></label>
      <div class="field">
        <span>{api.inWeb() ? "Log path on server host (blank = default)" : "Log path (blank = default)"}</span>
        <input type="text" bind:value={logPath} />
      </div>
    {/if}

    {#if section === "all" || section === "application"}
      <label class="t"><span>Hide IP addresses</span><input type="checkbox" bind:checked={hideIPs} /></label>
      <label class="t"><span>Check for updates on startup</span><input type="checkbox" bind:checked={updateCheck} /></label>
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
  .text-color-settings {
    margin: 0 0 8px;
    padding: 8px;
    border: 1px solid var(--border);
  }
  .text-color-settings legend { color: var(--accent); font-size: 12px; font-weight: 700; }
  .text-color-settings p { margin: 6px 4px 10px; color: var(--fg-dim); font-size: 11px; line-height: 1.45; }
  .related-settings { display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
  .related-settings button { display: flex; align-items: center; gap: 8px; padding: 8px; text-align: left; }
  .related-settings button span { flex: 1; min-width: 0; }
  .related-settings strong, .related-settings small { display: block; }
  .related-settings small { margin-top: 2px; color: var(--fg-dim); font-size: 10px; }
  .related-settings b { color: var(--accent); }
  .wound-settings {
    margin: 8px 0;
    padding: 8px;
    border: 1px solid var(--border);
  }
  .wound-settings legend {
    color: var(--fg-dim);
    font-size: 13px;
    font-weight: 600;
  }
  .wound-settings p {
    margin: 8px 4px 2px;
    color: var(--fg-dim);
    font-size: 12px;
  }
  .wound-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px 12px;
    padding: 8px 4px 2px;
  }
  .wound-grid.disabled {
    opacity: 0.55;
  }
  .wound-grid label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    color: var(--fg-dim);
    font-size: 12px;
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
