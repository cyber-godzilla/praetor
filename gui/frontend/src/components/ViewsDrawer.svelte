<script lang="ts">
  import type { DisplaySidebarConfig } from "../lib/types";
  import { store } from "../lib/store.svelte";
  import SidebarColumn from "./SidebarColumn.svelte";

  let {
    sidebarIDs,
    onsave,
  }: {
    sidebarIDs: string[];
    onsave: () => void;
  } = $props();

  const selectedID = $derived(sidebarIDs.includes(store.viewsDrawerSidebarID) ? store.viewsDrawerSidebarID : (sidebarIDs[0] ?? ""));
  const sidebar = $derived<DisplaySidebarConfig | undefined>(store.displayProfile()?.Sidebars?.find((item) => item.ID === selectedID));
</script>

<div class="backdrop" role="presentation" onclick={(event) => { if (event.target === event.currentTarget) store.viewsDrawerOpen = false; }}>
  <div class="drawer" role="dialog" aria-label="Sidebar views">
    <header>
      <div class="choices">
        {#each sidebarIDs as id (id)}
          {@const item = store.displayProfile()?.Sidebars.find((sidebar) => sidebar.ID === id)}
          <button class:active={selectedID === id} onclick={() => (store.viewsDrawerSidebarID = id)}>{item?.Name ?? "Sidebar"}</button>
        {/each}
      </div>
      <button class="close" aria-label="Close Views" onclick={() => (store.viewsDrawerOpen = false)}>✕</button>
    </header>
    {#if sidebar}
      <SidebarColumn {sidebar} drawer {onsave} />
    {/if}
  </div>
</div>

<style>
  .backdrop { position: fixed; z-index: 80; inset: 0; background: rgba(0,0,0,.62); display: flex; justify-content: flex-end; }
  .drawer { width: min(92vw, 520px); height: 100%; display: flex; flex-direction: column; background: var(--bg-panel); border-left: 1px solid var(--accent-dim); }
  header { flex: 0 0 auto; display: flex; gap: 8px; align-items: center; padding: 8px; border-bottom: 1px solid var(--border); }
  .choices { flex: 1; display: flex; gap: 4px; }
  .choices button { flex: 1; }
  .choices button.active { border-color: var(--accent); color: var(--accent); }
  .close { width: 38px; }
</style>
