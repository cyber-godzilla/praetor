<script lang="ts">
  import { store } from "../lib/store.svelte";

  const tabs = $derived(store.tabs.filter((t) => t.visible));

  function select(name: string) {
    store.activeTab = store.tabs.findIndex((t) => t.name === name);
  }
</script>

<div class="tabbar">
  {#each tabs as tab (tab.name)}
    <button
      class="tab"
      class:active={store.tabs[store.activeTab]?.name === tab.name}
      onclick={() => select(tab.name)}
    >
      {tab.name}
    </button>
  {/each}
  <div class="spacer"></div>
  <button class="tab menu-btn" title="Menu (Esc)" onclick={() => (store.openModal = "menu")}>☰</button>
</div>

<style>
  .tabbar {
    display: flex;
    align-items: stretch;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    padding: 0 4px;
    gap: 2px;
    min-height: 34px;
  }
  .tab {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    color: var(--fg-dim);
    padding: 6px 14px;
    font-size: 13px;
  }
  .tab:hover {
    color: var(--fg);
    background: var(--bg-elevated);
  }
  .tab.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }
  .menu-btn {
    font-size: 16px;
    padding: 6px 12px;
  }
</style>
