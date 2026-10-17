<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { ActionSet } from "../../lib/types";
  import { newActionSetID } from "../../lib/actions";

  let name = $state("");
  let saving = $state(false);
  const valid = $derived(name.trim().length > 0);

  async function save() {
    if (!valid || saving) return; // guard invalid input and double-submit
    saving = true;
    const sets: ActionSet[] = [
      ...(store.characterProfile()?.ActionSets ?? []),
      { ID: newActionSetID(), Name: name.trim(), Buttons: [] },
    ];
    try {
      await api.setActionSets(sets);
      store.characterProfile()!.ActionSets = sets;
      const created = sets[sets.length - 1];
      store.selectedActionSetID = created.ID;
      store.characterProfile()!.SelectedActionSetID = created.ID;
      await api.setSelectedActionSetID(created.ID);
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    } finally {
      saving = false;
    }
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
