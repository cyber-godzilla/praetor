<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  // Dedupe: "disable" is always offered first, but a loaded script may also
  // define a mode literally named "disable" — a duplicate key would crash the
  // keyed {#each} below (each_key_duplicate) and the modal would never render.
  const modes = $derived([...new Set(["disable", ...(store.modeNames ?? [])])]);

  // Each mode's declared metadata, keyed for lookup while rendering. Modes that
  // declare nothing (and the built-in "disable") simply have no entry.
  const specs = $derived(
    new Map((store.modeSpecs ?? []).map((s) => [s.name, s])),
  );

  // Pull the current mode list on open so it reflects any scripts loaded/
  // reloaded since startup, rather than the initial snapshot. The specs are
  // refreshed alongside the names so a reloaded script's new usage/desc shows
  // up here and in the command hint without a restart.
  onMount(async () => {
    const [names, loaded] = await Promise.all([api.modeNames(), api.modeSpecs()]);
    if (names && names.length) store.modeNames = names;
    if (loaded) store.modeSpecs = loaded;
  });

  async function pick(mode: string) {
    try {
      await api.setMode(mode, []);
      store.openModal = null;
    } catch (e) {
      store.addToast("Mode error", String(e));
    }
  }
</script>

<Modal title="Switch Mode" wide>
  {#if (store.modeNames ?? []).length === 0}
    <p class="dim empty">No Lua modes loaded. Add script directories in the menu, then reload scripts.</p>
  {/if}
  <div class="modes">
    {#each modes as mode (mode)}
      <button class="mode" class:current={store.mode === mode || (mode === "disable" && (store.mode === "" || store.mode === "disable"))} onclick={() => pick(mode)} disabled={!store.transportReady}>
        <span class="head">
          <span class="name">{mode}</span>
          {#if specs.get(mode)?.usage}
            <span class="usage">{specs.get(mode)?.usage}</span>
          {/if}
          {#if specs.get(mode)?.chains}
            <span class="usage">[after_&lt;mode|do|ps&gt;:&lt;mode|command|praetorscript&gt;]</span>
          {/if}
        </span>
        {#if specs.get(mode)?.desc}
          <span class="desc">{specs.get(mode)?.desc}</span>
        {/if}
        {#if store.mode === mode || (mode === "disable" && (store.mode === "" || store.mode === "disable"))}
          <span class="badge">active</span>
        {/if}
      </button>
    {/each}
  </div>
</Modal>

<style>
  .empty {
    margin: 0 0 12px;
    font-size: 13px;
  }
  .modes {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  /* Two columns: the mode's own text on the left, the active badge pinned
     right. Rows stack name-and-signature above the description, so a mode that
     declares neither collapses back to exactly the old single-line button. */
  .mode {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      "head badge"
      "desc badge";
    align-items: center;
    gap: 2px 10px;
    padding: 11px 14px;
    text-align: left;
    font-size: 14px;
  }
  .mode.current {
    border-color: var(--accent);
    color: var(--accent);
  }
  .head {
    grid-area: head;
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 8px;
    min-width: 0;
  }
  .name {
    font-family: var(--mono);
  }
  .usage {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--fg-dim);
  }
  .desc {
    grid-area: desc;
    font-size: 12px;
    color: var(--fg-dim);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badge {
    grid-area: badge;
    font-size: 11px;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
</style>
