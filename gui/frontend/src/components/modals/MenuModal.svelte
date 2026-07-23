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
        { label: "Action Sets", go: "actionsets" },
        { label: "Notifications", go: "notifications" },
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
        { label: "Notes", go: "notes" },
        { label: "Help", go: "help" },
      ],
    },
    {
      name: "Session",
      items: api.inWeb()
        ? [
            { label: "Disconnect shared game", action: logout },
            { label: "Sign out of web UI", action: signOut },
          ]
        : [
            { label: "Logout", action: logout },
            { label: "Exit", action: () => void api.quit() },
          ],
    },
  ];

  async function reloadScripts() {
    // Stay in the menu after reloading so the user can keep working.
    try {
      await api.reloadScripts();
      store.modeNames = await api.modeNames();
      store.addToast("Scripts reloaded", "");
    } catch (e) {
      store.addToast("Reload failed", String(e));
    }
  }

  async function logout() {
    if (
      api.inWeb() &&
      !window.confirm("Disconnect the shared TEC game session for every connected browser?")
    ) {
      return;
    }
    // Close the menu immediately; the resulting disconnected event drives the
    // screen back to the bootup screen.
    store.openModal = null;
    try {
      await api.disconnect();
    } catch (error) {
      store.addToast("Disconnect failed", error instanceof Error ? error.message : String(error));
    }
  }

  async function signOut() {
    store.openModal = null;
    try {
      await api.quit();
    } catch (error) {
      store.addToast("Sign out failed", error instanceof Error ? error.message : String(error));
    }
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
