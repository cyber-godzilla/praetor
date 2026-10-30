<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { ActionSet } from "../../lib/types";
  import { newActionSetID } from "../../lib/actions";

  function clone(s: ActionSet): ActionSet {
    return { ID: s.ID, Name: s.Name, Buttons: (s.Buttons ?? []).map((b) => ({ ...b })) };
  }

  const profile = store.characterProfile();
  const isDefault = profile?.ID === store.config?.CharacterProfiles.DefaultID;
  let inheriting = $state(!isDefault && profile?.ActionSetsMode === "inherit");
  let sets = $state<ActionSet[]>(store.resolvedActionSets().map(clone));
  let sel = $state(0);
  let newSetName = $state("");
  let sharedID = $state("");
  let saving = $state(false);

  const current = $derived(sets[sel]);
  const sharedChoices = $derived.by(() => {
    const byID = new Map<string, ActionSet>();
    for (const candidate of store.config?.CharacterProfiles.Profiles ?? []) {
      for (const set of candidate.ActionSets ?? []) byID.set(set.ID, set);
    }
    return [...byID.values()].filter((set) => !sets.some((owned) => owned.ID === set.ID));
  });

  function customize() {
    inheriting = false;
    sets = store.resolvedActionSets().map(clone);
    sel = 0;
  }

  function addShared() {
    const source = sharedChoices.find((set) => set.ID === sharedID);
    if (!source) return;
    sets = [...sets, clone(source)];
    sel = sets.length - 1;
    sharedID = "";
  }

  function copyCurrent() {
    if (!current) return;
    sets = [...sets, { ...clone(current), ID: newActionSetID(), Name: `${current.Name} copy` }];
    sel = sets.length - 1;
  }

  async function deleteEverywhere() {
    if (!current) return;
    const usedBy = (store.config?.CharacterProfiles.Profiles ?? [])
      .filter((candidate) => (candidate.ActionSets ?? []).some((set) => set.ID === current.ID))
      .map((candidate) => candidate.Name);
    if (!confirm(`Delete “${current.Name}” everywhere?\n\nUsed by: ${usedBy.join(", ") || "this profile"}`)) return;
    try {
      await api.deleteActionSetEverywhere(current.ID);
      for (const candidate of store.config?.CharacterProfiles.Profiles ?? []) {
        candidate.ActionSets = (candidate.ActionSets ?? []).filter((set) => set.ID !== current.ID);
      }
      sets = sets.filter((set) => set.ID !== current.ID);
      sel = Math.min(sel, Math.max(0, sets.length - 1));
    } catch (e) {
      store.addToast("Delete failed", String(e));
    }
  }

  function addSet() {
    const n = newSetName.trim();
    if (!n) return;
    sets = [...sets, { ID: newActionSetID(), Name: n, Buttons: [] }];
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
      ID: s.ID,
      Name: s.Name,
      Buttons: (s.Buttons ?? []).filter((b) => b.Label.trim() !== "" || b.Command.trim() !== ""),
    }));
    try {
      if (inheriting) {
        await api.setActionSetInheritance(true);
        store.characterProfile()!.ActionSetsMode = "inherit";
      } else {
        await api.setActionSets(cleaned);
        store.characterProfile()!.ActionSetsMode = "customize";
        store.characterProfile()!.ActionSets = cleaned;
      }
      // Deletes/reorders here can leave the sidebar's index pointing at a
      // different set; reset it so it can't mis-target.
      const selected = cleaned.find((set) => set.ID === store.selectedActionSetID) ?? cleaned[0];
      store.selectedActionSetID = selected?.ID ?? "";
      store.characterProfile()!.SelectedActionSetID = selected?.ID ?? "";
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
  {#if !isDefault}
    <div class="scope">
      <label><input type="radio" name="action-scope" checked={inheriting} onchange={() => (inheriting = true)} /> Inherit Default</label>
      <label><input type="radio" name="action-scope" checked={!inheriting} onchange={customize} /> Customize for {profile?.Name}</label>
    </div>
  {/if}
  {#if inheriting}
    <p class="dim inherited-note">These Action Sets follow Default live. Choose Customize to change membership for this character.</p>
  {/if}
  <div class="cols">
    <div class="setlist">
      {#each sets as s, i (i)}
        <div class="srow" class:active={sel === i}>
          <button class="sname" onclick={() => (sel = i)}>{s.Name}</button>
          <button class="danger sm" onclick={() => removeSet(i)} disabled={inheriting} title="Remove from this profile">✕</button>
        </div>
      {/each}
      <div class="row addset" class:hidden={inheriting}>
        <input type="text" bind:value={newSetName} placeholder="New set…"
          onkeydown={(e) => {
            if ((e.key === "Enter" || e.key === "Tab") && newSetName.trim()) {
              e.preventDefault();
              addSet();
            }
          }} />
        <button onclick={addSet}>+</button>
      </div>
      {#if !inheriting && sharedChoices.length > 0}
        <div class="row share">
          <select bind:value={sharedID} aria-label="Existing Action Set">
            <option value="">Add existing…</option>
            {#each sharedChoices as set (set.ID)}<option value={set.ID}>{set.Name}</option>{/each}
          </select>
          <button onclick={addShared} disabled={!sharedID}>Add</button>
        </div>
      {/if}
    </div>

    <div class="editor">
      {#if current}
        <label class="setname">Set name
          <input type="text" bind:value={current.Name} placeholder="Set name" disabled={inheriting} />
        </label>
        {#if !inheriting}<div class="row"><button class="sm" onclick={copyCurrent}>Copy with new ID</button><button class="danger sm" onclick={deleteEverywhere}>Delete everywhere…</button></div>{/if}
        <div class="btnshead dim">Buttons</div>
        {#each current.Buttons ?? [] as btn, i (i)}
          <div class="brow">
            <input type="text" class="lbl" bind:value={btn.Label} placeholder="Label" disabled={inheriting} />
            <input type="text" class="cmd" bind:value={btn.Command} placeholder="command" disabled={inheriting} />
            <button class="sm" onclick={() => moveButton(i, -1)} title="Move up">↑</button>
            <button class="sm" onclick={() => moveButton(i, 1)} title="Move down">↓</button>
            <button class="danger sm" onclick={() => removeButton(i)}>✕</button>
          </div>
        {/each}
        <button class="sm" onclick={addButton} disabled={inheriting}>+ Add button</button>
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
  .scope { display: flex; gap: 18px; margin-bottom: 10px; font-size: 12px; }
  .scope label { display: flex; align-items: center; gap: 6px; }
  .inherited-note { margin: 0 0 10px; font-size: 12px; }
  .hidden { display: none; }
  .share select { flex: 1; min-width: 0; }
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
