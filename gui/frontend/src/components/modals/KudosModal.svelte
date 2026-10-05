<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { KudosConfig } from "../../lib/types";

  let { loginPrompt = false }: { loginPrompt?: boolean } = $props();

  const src = store.config?.Kudos;
  let kudos = $state<KudosConfig>({
    Favorites: [...(src?.Favorites ?? [])],
    Queue: (src?.Queue ?? []).map((q) => ({ ...q })),
  });

  let newFav = $state("");
  let newName = $state("");
  let newMsg = $state("");

  function addFav() {
    const n = newFav.trim();
    if (n && !(kudos.Favorites ?? []).includes(n)) kudos.Favorites = [...(kudos.Favorites ?? []), n];
    newFav = "";
  }
  function removeFav(i: number) {
    kudos.Favorites = (kudos.Favorites ?? []).filter((_, idx) => idx !== i);
  }
  function addQueue() {
    const n = newName.trim();
    if (!n) return;
    kudos.Queue = [...(kudos.Queue ?? []), { Name: n, Message: newMsg.trim() }];
    newName = "";
    newMsg = "";
  }
  function removeQueue(i: number) {
    kudos.Queue = (kudos.Queue ?? []).filter((_, idx) => idx !== i);
  }

  function prefillFavorite(name: string) {
    store.inputPrefill = "@kudos " + name;
    persistThenClose();
  }
  function sendQueued(i: number) {
    const e = (kudos.Queue ?? [])[i];
    if (!e) return;
    api.send(`@kudos ${e.Name} ${e.Message}`);
    kudos.Queue = (kudos.Queue ?? []).filter((_, idx) => idx !== i);
  }

  async function persist(): Promise<boolean> {
    try {
      await api.setKudos(kudos);
      store.config!.Kudos = kudos;
      return true;
    } catch (e) {
      store.addToast("Save failed", String(e));
      return false;
    }
  }
  async function persistThenClose() {
    if (await persist()) store.openModal = null;
  }
</script>

<Modal title={loginPrompt ? "Kudos — queued entries" : "Kudos"} wide back onsave={persistThenClose}>
  {#if loginPrompt}
    <p class="hint dim">You have queued kudos. Send them now or manage the list.</p>
  {/if}

  <div class="phead dim">Favorites <span class="sub">(click to prefill the input)</span></div>
  <div class="list">
    {#each kudos.Favorites ?? [] as name, i (name)}
      <div class="item">
        <button class="link" onclick={() => prefillFavorite(name)}>{name}</button>
        <button class="danger sm" onclick={() => removeFav(i)}>✕</button>
      </div>
    {/each}
    {#if (kudos.Favorites ?? []).length === 0}<div class="dim empty">none</div>{/if}
  </div>
  <div class="row add">
    <input type="text" bind:value={newFav} placeholder="Add favorite…" onkeydown={(e) => e.key === "Enter" && addFav()} />
    <button onclick={addFav}>Add</button>
  </div>

  <div class="phead dim">Queue</div>
  <div class="list">
    {#each kudos.Queue ?? [] as e, i (i)}
      <div class="item">
        <span class="qn">{e.Name}</span>
        <span class="qm dim">{e.Message}</span>
        <span class="spacer"></span>
        <button class="primary sm" onclick={() => sendQueued(i)}>Send</button>
        <button class="danger sm" onclick={() => removeQueue(i)}>✕</button>
      </div>
    {/each}
    {#if (kudos.Queue ?? []).length === 0}<div class="dim empty">none</div>{/if}
  </div>
  <div class="row add">
    <input type="text" bind:value={newName} placeholder="Name" />
    <input type="text" bind:value={newMsg} placeholder="Message" />
    <button onclick={addQueue}>Queue</button>
  </div>

</Modal>

<style>
  .hint {
    margin: 0 0 12px;
  }
  .phead {
    margin: 14px 0 8px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .phead .sub {
    text-transform: none;
    letter-spacing: 0;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    background: var(--bg-elevated);
    border-radius: 5px;
  }
  .link {
    background: none;
    border: none;
    color: var(--accent);
    padding: 0;
  }
  .link:hover {
    text-decoration: underline;
    background: none;
  }
  .qn {
    font-weight: 600;
  }
  .sm {
    padding: 3px 9px;
    font-size: 12px;
  }
  .add {
    margin-top: 6px;
  }
  .add input {
    flex: 1;
    min-width: 0;
  }
  .empty {
    padding: 8px;
  }
</style>
