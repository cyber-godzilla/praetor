<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  // Buffered like the Highlights editor: edit locally, apply on Save, discard
  // on Cancel.
  let scanlines = $state(untrack(() => store.config?.UI?.CRTScanlines ?? true));
  let roll = $state(untrack(() => store.config?.UI?.CRTRoll ?? true));
  let bloom = $state(untrack(() => store.config?.UI?.CRTBloom ?? true));

  async function save() {
    try {
      await api.setCRTEffects(scanlines, roll, bloom);
      if (store.config) {
        store.config.UI.CRTScanlines = scanlines;
        store.config.UI.CRTRoll = roll;
        store.config.UI.CRTBloom = bloom;
      }
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title="Retro CRT Effects" back onsave={save}>
  <p class="hint dim">Toggle each effect on or off.</p>
  <div class="toggles">
    <label class="t"><span>Scanlines &amp; vignette</span><input type="checkbox" bind:checked={scanlines} /></label>
    <label class="t"><span>Rolling band (motion)</span><input type="checkbox" bind:checked={roll} /></label>
    <label class="t"><span>Phosphor bloom</span><input type="checkbox" bind:checked={bloom} /></label>
  </div>
</Modal>

<style>
  .hint {
    margin: 0 0 10px;
    font-size: 12px;
  }
  .toggles {
    display: flex;
    flex-direction: column;
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
</style>
