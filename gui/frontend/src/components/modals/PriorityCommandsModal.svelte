<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  const profile = store.characterProfile();
  const isDefault = profile?.ID === store.config?.CharacterProfiles.DefaultID;
  let inheriting = $state(!isDefault && profile?.PriorityCommandsMode === "inherit");
  let items = $state<string[]>([...store.resolvedHighPriority()]);
  let draft = $state("");

  function customize() {
    inheriting = false;
    items = [...store.resolvedHighPriority()];
  }
  function add() {
    const value = draft.trim();
    if (value && !items.includes(value)) items = [...items, value];
    draft = "";
  }

  async function save() {
    try {
      if (inheriting) {
        await api.setHighPriorityInheritance(true);
        store.characterProfile()!.PriorityCommandsMode = "inherit";
      } else {
        await api.setHighPriority(items);
        if (isDefault) store.config!.Commands.HighPriority = items;
        else {
          store.characterProfile()!.PriorityCommandsMode = "customize";
          store.characterProfile()!.HighPriority = items;
        }
      }
      store.addToast("High-priority commands", "Saved");
      store.openModal = null;
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
  }
</script>

<Modal title="High-priority commands" back onsave={save}>
  <p class="hint dim">Commands with immediate precedence during script events, bypassing other queued commands.</p>
  {#if !isDefault}
    <div class="scope">
      <label><input type="radio" name="priority-scope" checked={inheriting} onchange={() => (inheriting = true)} /> Inherit Default</label>
      <label><input type="radio" name="priority-scope" checked={!inheriting} onchange={customize} /> Customize</label>
    </div>
  {/if}
  <div class="list">
    {#each items as item, i (item)}
      <div class="item"><span>{item}</span><button class="danger" onclick={() => (items = items.filter((_, index) => index !== i))} disabled={inheriting}>Remove</button></div>
    {/each}
    {#if items.length === 0}<div class="dim empty">Empty</div>{/if}
  </div>
  <div class="row">
    <input bind:value={draft} placeholder="Add…" disabled={inheriting} onkeydown={(event) => event.key === "Enter" && add()} />
    <button onclick={add} disabled={inheriting}>Add</button>
  </div>
</Modal>

<style>
  .hint { margin: 0 0 12px; font-size: 12px; }
  .scope { display: flex; gap: 18px; margin-bottom: 12px; font-size: 12px; }
  .list { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }
  .item { display: flex; align-items: center; justify-content: space-between; padding: 7px 10px; background: var(--bg-elevated); border-radius: 5px; font: 13px var(--mono); }
  .item button { padding: 3px 8px; font-size: 12px; }
  .row { display: flex; gap: 6px; }
  .row input { flex: 1; }
  .empty { padding: 12px; text-align: center; }
</style>
