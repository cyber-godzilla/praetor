<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { NoteSummary } from "../../lib/types";

  let view = $state<"list" | "edit">("list");
  let summaries = $state<NoteSummary[]>([]);

  // Editor state
  let originalTitle = $state(""); // "" = new note
  let title = $state("");
  let body = $state("");
  let dirty = $state(false);
  let confirmDiscard = $state(false);
  let confirmDelete = $state(false);
  let confirmDeleteTitle = $state(""); // which list card's delete is being confirmed

  async function refresh() {
    try {
      summaries = await api.listNotes();
    } catch (e) {
      store.addToast("Notes", String(e));
    }
  }

  function openEditor(o: { originalTitle: string; title: string; body: string }) {
    originalTitle = o.originalTitle;
    title = o.title;
    body = o.body;
    dirty = false;
    confirmDiscard = false;
    confirmDelete = false;
    view = "edit";
    store.notesEditorActive = true;
  }

  function toList() {
    view = "list";
    confirmDiscard = false;
    confirmDelete = false;
    store.notesEditorActive = false;
    refresh();
  }

  async function openExisting(t: string) {
    try {
      const n = await api.getNote(t);
      openEditor({ originalTitle: n.title, title: n.title, body: n.body });
    } catch (e) {
      store.addToast("Notes", String(e));
    }
  }

  // Guarded return to list (Back button and Esc route here).
  function requestList() {
    if (dirty) {
      confirmDiscard = true;
      confirmDelete = false;
    } else toList();
  }

  async function save() {
    try {
      await api.saveNote(originalTitle, title, body);
      store.addToast("Notes", "Saved");
      dirty = false;
      toList();
    } catch (e) {
      store.addToast("Notes", String(e));
    }
  }

  async function del(t: string) {
    try {
      await api.deleteNote(t);
      store.addToast("Notes", `Deleted "${t}".`);
    } catch (e) {
      store.addToast("Notes", String(e));
    }
    confirmDelete = false;
    if (view === "edit") toList();
    else refresh();
  }

  // Close-guard passed to Modal: veto (and prompt) only for a dirty editor.
  function guard(): boolean {
    if (view === "edit" && dirty) {
      confirmDiscard = true;
      confirmDelete = false;
      return false;
    }
    return true;
  }

  // GameView bumps notesBackRequest for Esc while the editor is active.
  let lastBack = store.notesBackRequest;
  $effect(() => {
    const r = store.notesBackRequest;
    if (r === lastBack) return;
    lastBack = r;
    if (view === "edit") requestList();
  });

  onMount(() => {
    const init = store.notesInitial ?? { view: "list" };
    store.notesInitial = null;
    if (init.view === "edit") {
      openEditor({ originalTitle: init.originalTitle, title: init.title, body: init.body });
    } else {
      view = "list";
      refresh();
    }
    return () => (store.notesEditorActive = false);
  });
</script>

{#if view === "list"}
  <Modal title="Notes" wide back>
    <div class="list">
      {#each summaries as n (n.title)}
        <div class="card">
          <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
          <div class="cardmain" role="button" tabindex="-1" onclick={() => openExisting(n.title)}>
            <div class="ntitle">{n.title}</div>
            <div class="npreview">{n.preview}</div>
          </div>
          {#if confirmDelete && confirmDeleteTitle === n.title}
            <span class="confirm">
              <button class="danger sm" onclick={() => del(n.title)}>Delete</button>
              <button class="sm" onclick={() => (confirmDelete = false)}>Cancel</button>
            </span>
          {:else}
            <button class="danger sm" title="Delete"
              onclick={() => { confirmDelete = true; confirmDeleteTitle = n.title; confirmDiscard = false; }}>✕</button>
          {/if}
        </div>
      {/each}
      {#if summaries.length === 0}<div class="dim empty">No notes yet.</div>{/if}
    </div>
    <div class="row">
      <button class="primary" onclick={() => openEditor({ originalTitle: "", title: "", body: "" })}>
        + New note
      </button>
    </div>
  </Modal>
{:else}
  <Modal title="Note" wide {guard}>
    <div class="editor">
      <button class="backlink" onclick={requestList}>‹ Notes</button>
      <input class="etitle" type="text" placeholder="Title" bind:value={title}
        oninput={() => (dirty = true)} />
      <textarea class="ebody" placeholder="Write your note…" bind:value={body}
        oninput={() => (dirty = true)}></textarea>
    </div>
    {#snippet footer()}
      {#if confirmDelete}
        <button class="danger" onclick={() => del(originalTitle)}>Confirm delete</button>
        <button onclick={() => (confirmDelete = false)}>Cancel</button>
      {:else if originalTitle}
        <button class="danger" onclick={() => { confirmDelete = true; confirmDiscard = false; }}>Delete</button>
      {/if}
      {#if confirmDiscard}
        <span class="confirm">
          <span class="dim">Discard unsaved changes?</span>
          <button class="danger sm" onclick={() => { dirty = false; toList(); }}>Discard</button>
          <button class="sm" onclick={() => (confirmDiscard = false)}>Keep editing</button>
        </span>
      {/if}
      <button class="primary" onclick={save}>Save</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 12px;
  }
  .card {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
  }
  .cardmain {
    flex: 1;
    cursor: pointer;
    min-width: 0;
  }
  .ntitle {
    font-weight: 700;
    font-size: 16px;
    color: var(--fg-bright);
  }
  .npreview {
    font-size: 13px;
    color: var(--fg-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .empty {
    padding: 16px;
    text-align: center;
  }
  .sm {
    padding: 4px 8px;
    font-size: 12px;
  }
  .confirm {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .editor {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .backlink {
    align-self: flex-start;
    font-size: 12px;
    color: var(--fg-dim);
    padding: 2px 6px;
  }
  .backlink:hover {
    color: var(--accent);
    border-color: var(--accent);
  }
  .etitle {
    font-size: 15px;
    font-weight: 700;
  }
  .ebody {
    min-height: 320px;
    resize: vertical;
    font-family: var(--mono);
    font-size: 13px;
    line-height: 1.5;
    background: var(--bg-input);
    border: 1px solid var(--border);
    padding: 8px;
  }
</style>
