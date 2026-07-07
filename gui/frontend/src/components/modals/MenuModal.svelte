<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  interface Item {
    label: string;
    go?: string;
    action?: () => void;
  }

  const groups: { name: string; items: Item[] }[] = [
    {
      name: "Display & Behavior",
      items: [
        { label: "Settings", go: "settings" },
        { label: "Retro CRT Effects", go: "crt" },
        { label: "Highlights", go: "highlights" },
        { label: "Custom Tabs", go: "tabs" },
        { label: "Notifications", go: "notifications" },
        { label: "Toggle Sidebar (Alt+S)", action: () => (store.sidebarOpen = !store.sidebarOpen) },
      ],
    },
    {
      name: "Automation",
      items: [
        { label: "Script Directories", go: "scripts" },
        { label: "Quick-Cycle Modes", go: "quickcycle" },
        { label: "High-Priority Commands", go: "priority" },
        { label: "Persistent Data", go: "persistent" },
        { label: "Reload Scripts", action: reloadScripts },
      ],
    },
    {
      name: "Filters",
      items: [
        { label: "Ignore OOC Accounts", go: "ignore-ooc" },
        { label: "Ignore Think Characters", go: "ignore-think" },
      ],
    },
    {
      name: "Tools & References",
      items: [
        { label: "Kudos", go: "kudos" },
        { label: "Rank-Bonus Calculator", go: "calc" },
        { label: "Wiki Bookmarks", go: "wiki" },
        { label: "Map Bookmarks", go: "maps" },
        { label: "Help", go: "help" },
      ],
    },
    {
      name: "Session",
      items: [
        { label: "Quit", action: () => window.runtime?.Quit() },
      ],
    },
  ];

  async function reloadScripts() {
    try {
      await api.reloadScripts();
      store.modeNames = await api.modeNames();
      store.addToast("Scripts reloaded", "");
    } catch (e) {
      store.addToast("Reload failed", String(e));
    }
    store.openModal = null;
  }

  function pick(it: Item) {
    if (it.action) it.action();
    else if (it.go) store.openModal = it.go;
  }
</script>

<Modal title="Menu">
  <div class="groups">
    {#each groups as g (g.name)}
      <div class="group">
        <div class="gname">{g.name}</div>
        <div class="items">
          {#each g.items as it (it.label)}
            <button class="mitem" onclick={() => pick(it)}>{it.label}</button>
          {/each}
        </div>
      </div>
    {/each}
  </div>
</Modal>

<style>
  .groups {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .gname {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--accent);
    margin-bottom: 6px;
  }
  .items {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }
  .mitem {
    text-align: left;
  }
</style>
