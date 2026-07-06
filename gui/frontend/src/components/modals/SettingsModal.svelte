<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  const ui = $derived(store.config?.UI);

  async function toggle(get: () => boolean, set: (v: boolean) => Promise<void>, apply: (v: boolean) => void) {
    const v = !get();
    apply(v);
    try {
      await set(v);
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title="Settings">
  {#if store.config}
    <div class="toggles">
      <label class="t">
        <span>Echo typed commands</span>
        <input type="checkbox" checked={ui?.EchoTyped}
          onchange={() => toggle(() => ui!.EchoTyped, api.setEchoTyped, (v) => (store.config!.UI.EchoTyped = v))} />
      </label>
      <label class="t">
        <span>Echo script commands</span>
        <input type="checkbox" checked={ui?.EchoScript}
          onchange={() => toggle(() => ui!.EchoScript, api.setEchoScript, (v) => (store.config!.UI.EchoScript = v))} />
      </label>
      <label class="t">
        <span>Color words</span>
        <input type="checkbox" checked={ui?.ColorWords}
          onchange={() => toggle(() => ui!.ColorWords, api.setColorWords, (v) => (store.config!.UI.ColorWords = v))} />
      </label>
      <label class="t">
        <span>Hide IP addresses</span>
        <input type="checkbox" checked={ui?.HideIPs}
          onchange={() => toggle(() => ui!.HideIPs, api.setHideIPs, (v) => (store.config!.UI.HideIPs = v))} />
      </label>
      <label class="t">
        <span>Auto-reconnect</span>
        <input type="checkbox" checked={store.config.Reconnect.Enabled}
          onchange={() => toggle(() => store.config!.Reconnect.Enabled, api.setAutoReconnect, (v) => (store.config!.Reconnect.Enabled = v))} />
      </label>
      <label class="t">
        <span>Session transcript logging</span>
        <input type="checkbox" checked={store.config.Logging.Session.Enabled}
          onchange={() => toggle(() => store.config!.Logging.Session.Enabled, api.setSessionLogging, (v) => (store.config!.Logging.Session.Enabled = v))} />
      </label>

      <div class="field">
        <span>Minimap scale</span>
        <input type="number" min="0.2" max="3" step="0.1" value={ui?.MinimapScale}
          onchange={(e) => {
            const v = parseFloat((e.target as HTMLInputElement).value);
            store.config!.UI.MinimapScale = v;
            api.setMinimapScale(v);
          }} />
      </div>

      <div class="field">
        <span>Log path (blank = default)</span>
        <input type="text" value={store.config.Logging.Session.Path}
          onchange={(e) => {
            const v = (e.target as HTMLInputElement).value;
            store.config!.Logging.Session.Path = v;
            api.setLogPath(v);
          }} />
      </div>
    </div>
  {/if}
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
  .t input {
    width: 16px;
    height: 16px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 10px 4px;
    font-size: 13px;
    color: var(--fg-dim);
  }
</style>
