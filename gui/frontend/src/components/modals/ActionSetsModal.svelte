<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { ActionSet } from "../../lib/types";

  function clone(s: ActionSet): ActionSet {
    return { Name: s.Name, Buttons: (s.Buttons ?? []).map((b) => ({ ...b })) };
  }

  let sets = $state<ActionSet[]>((store.config?.UI?.ActionSets ?? []).map(clone));
  let sel = $state(0);
  let newSetName = $state("");
  let saving = $state(false);

  const current = $derived(sets[sel]);

  function addSet() {
    const n = newSetName.trim();
    if (!n) return;
    sets = [...sets, { Name: n, Buttons: [] }];
    sel = sets.length - 1;
    newSetName = "";
  }
  function removeSet(i: number) {
    sets = sets.filter((_, idx) => idx !== i);
    // Keep `sel` on the same set: shift down if an earlier one was removed,
    // then clamp so it never dangles past the end (else edits/Save would land
    // on the wrong set).
    if (sel > i) sel -= 1;
    if (sel >= sets.length) sel = Math.max(0, sets.length - 1);
  }
  function addButton() {
    if (!current) return;
    current.Buttons = [...(current.Buttons ?? []), { Label: "", Command: "" }];
  }
  function removeButton(i: number) {
    if (!current) return;
    current.Buttons = (current.Buttons ?? []).filter((_, idx) => idx !== i);
  }
  function moveButton(i: number, dir: -1 | 1) {
    if (!current) return;
    const b = [...(current.Buttons ?? [])];
    const j = i + dir;
    if (j < 0 || j >= b.length) return;
    [b[i], b[j]] = [b[j], b[i]];
    current.Buttons = b;
  }

  async function save() {
    if (saving) return; // guard against double-submit
    saving = true;
    // Drop fully-blank buttons so they don't render as empty tiles that send an
    // empty command.
    const cleaned = sets.map((s) => ({
      Name: s.Name,
      Buttons: (s.Buttons ?? []).filter((b) => b.Label.trim() !== "" || b.Command.trim() !== ""),
    }));
    try {
      await api.setActionSets(cleaned);
      store.config!.UI.ActionSets = cleaned;
      // Deletes/reorders here can leave the sidebar's index pointing at a
      // different set; reset it so it can't mis-target.
      store.actionSetIndex = 0;
      store.addToast("Action sets", "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    } finally {
      saving = false;
    }
  }
</script>

<Modal title="Action Sets" wide back onsave={save}>
  <div class="cols">
    <div class="setlist">
      {#each sets as s, i (i)}
        <div class="srow" class:active={sel === i}>
          <button class="sname" onclick={() => (sel = i)}>{s.Name}</button>
          <button class="danger sm" onclick={() => removeSet(i)}>✕</button>
        </div>
      {/each}
      <div class="row addset">
        <input type="text" bind:value={newSetName} placeholder="New set…"
          onkeydown={(e) => {
            if ((e.key === "Enter" || e.key === "Tab") && newSetName.trim()) {
              e.preventDefault();
              addSet();
            }
          }} />
        <button onclick={addSet}>+</button>
      </div>
    </div>

    <div class="editor">
      {#if current}
        <label class="setname">Set name
          <input type="text" bind:value={current.Name} placeholder="Set name" />
        </label>
        <div class="btnshead dim">Buttons</div>
        {#each current.Buttons ?? [] as btn, i (i)}
          <div class="brow">
            <input type="text" class="lbl" bind:value={btn.Label} placeholder="Label" />
            <input type="text" class="cmd" bind:value={btn.Command} placeholder="command" />
            <button class="sm" onclick={() => moveButton(i, -1)} title="Move up">↑</button>
            <button class="sm" onclick={() => moveButton(i, 1)} title="Move down">↓</button>
            <button class="danger sm" onclick={() => removeButton(i)}>✕</button>
          </div>
        {/each}
        <button class="sm" onclick={addButton}>+ Add button</button>
      {:else}
        <div class="dim empty">Add a set to begin.</div>
      {/if}
    </div>
  </div>
</Modal>

<style>
  .cols {
    display: flex;
    gap: 16px;
    min-height: 260px;
  }
  .setlist {
    width: 180px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    border-right: 1px solid var(--border);
    padding-right: 12px;
  }
  .srow {
    display: flex;
    gap: 4px;
  }
  .srow .sname {
    flex: 1;
    text-align: left;
  }
  .srow.active .sname {
    border-color: var(--accent);
    color: var(--accent);
  }
  .addset {
    margin-top: 6px;
  }
  .addset input {
    flex: 1;
    min-width: 0;
  }
  .editor {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .btnshead {
    margin-top: 2px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .setname {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
  }
  .setname input {
    font: inherit;
  }
  .brow {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .brow .lbl {
    width: 90px;
  }
  .brow .cmd {
    flex: 1;
    min-width: 0;
  }
  .sm {
    padding: 4px 10px;
    font-size: 12px;
  }
  .empty {
    padding: 20px;
  }
  .row {
    display: flex;
    gap: 4px;
  }
</style>
