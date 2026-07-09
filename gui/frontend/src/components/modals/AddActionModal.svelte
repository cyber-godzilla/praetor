<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { ActionSet } from "../../lib/types";

  let label = $state("");
  let command = $state("");
  const valid = $derived(label.trim().length > 0 && command.trim().length > 0);

  async function save() {
    if (!valid) return;
    // Deep-clone so we don't mutate the reactive store objects before persisting.
    const sets: ActionSet[] = (store.config?.UI?.ActionSets ?? []).map((s) => ({
      Name: s.Name,
      Buttons: (s.Buttons ?? []).map((b) => ({ ...b })),
    }));
    const target = sets[store.actionSetIndex];
    if (!target) {
      store.openModal = null;
      return;
    }
    target.Buttons = [...(target.Buttons ?? []), { Label: label.trim(), Command: command.trim() }];
    try {
      await api.setActionSets(sets);
      if (store.config) store.config.UI.ActionSets = sets;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="New Action" onsave={save}>
  <label class="fld">Label
    <input type="text" bind:value={label} placeholder="e.g. Attack" />
  </label>
  <label class="fld">Command
    <input
      type="text"
      bind:value={command}
      placeholder="e.g. attack"
      onkeydown={(e) => {
        if (e.key === "Enter" && valid) {
          e.preventDefault();
          save();
        }
      }} />
  </label>
</Modal>

<style>
  .fld {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 13px;
    margin-bottom: 10px;
  }
  .fld input {
    font: inherit;
  }
</style>
