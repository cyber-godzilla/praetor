<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { ActionSet } from "../../lib/types";

  let name = $state("");
  const valid = $derived(name.trim().length > 0);

  async function save() {
    if (!valid) return;
    const sets: ActionSet[] = [
      ...(store.config?.UI?.ActionSets ?? []),
      { Name: name.trim(), Buttons: [] },
    ];
    try {
      await api.setActionSets(sets);
      if (store.config) store.config.UI.ActionSets = sets;
      store.actionSetIndex = sets.length - 1; // show the new set
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="New Action Set" onsave={save}>
  <label class="fld">Set name
    <input
      type="text"
      bind:value={name}
      placeholder="e.g. Combat"
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
  }
  .fld input {
    font: inherit;
  }
</style>
