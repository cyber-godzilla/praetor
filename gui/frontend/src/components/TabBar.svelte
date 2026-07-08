<script lang="ts">
  import { store } from "../lib/store.svelte";

  const tabs = $derived(store.tabs.filter((t) => t.visible));

  function select(name: string) {
    store.selectTab(store.tabs.findIndex((t) => t.name === name));
  }
</script>

<div class="tabbar">
  {#each tabs as tab (tab.name)}
    <button
      class="tab"
      class:active={store.tabs[store.activeTab]?.name === tab.name}
      onclick={() => select(tab.name)}
      tabindex="-1"
    >
      {tab.name}
      {#if tab.unread && store.tabs[store.activeTab]?.name !== tab.name}
        <span class="unread" title="New activity">●</span>
      {/if}
    </button>
  {/each}
  <div class="spacer"></div>
  <button class="tab menu-btn" title="Menu (Esc)" onclick={() => (store.openModal = "menu")} tabindex="-1">☰</button>
</div>

<style>
  .tabbar {
    display: flex;
    align-items: stretch;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    padding: 2px 2px 0;
    gap: 2px;
  }
  /* Active tab mirrors the TUI: orange background, dark bold text. */
  .tab {
    background: none;
    border: none;
    color: var(--fg-dim);
    padding: 4px 12px;
    font-size: 13px;
  }
  .tab:hover {
    color: var(--fg-bright);
    background: var(--bg-elevated);
  }
  .tab.active {
    background: var(--accent);
    color: #000;
    font-weight: 700;
  }
  .tab.active:hover {
    color: #000;
  }
  .unread {
    color: var(--accent);
    font-size: 8px;
    vertical-align: middle;
    margin-left: 5px;
  }
  .menu-btn {
    font-size: 15px;
    padding: 4px 12px;
  }
</style>
