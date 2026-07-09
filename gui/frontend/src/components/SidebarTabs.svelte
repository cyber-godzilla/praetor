<script lang="ts">
  import ActionsTab from "./sidebar/ActionsTab.svelte";
  import ModesTab from "./sidebar/ModesTab.svelte";

  type TabId = "actions" | "modes";
  const tabs: { id: TabId; label: string }[] = [
    { id: "actions", label: "Actions" },
    { id: "modes", label: "Modes" },
  ];

  // Default = Actions; component-local, resets on relaunch (not persisted).
  let active = $state<TabId>("actions");
</script>

<div class="sidebartabs">
  <div class="strip">
    {#each tabs as t (t.id)}
      <button class="tab" class:on={active === t.id} onclick={() => (active = t.id)} tabindex="-1">{t.label}</button>
    {/each}
  </div>
  <div class="content">
    {#if active === "actions"}
      <ActionsTab />
    {:else}
      <ModesTab />
    {/if}
  </div>
</div>

<style>
  .sidebartabs {
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
  }
  .strip {
    display: flex;
    gap: 2px;
    border-bottom: 1px solid var(--border);
    margin-bottom: 8px;
  }
  .tab {
    flex: 1;
    padding: 6px 4px;
    font-size: 12px;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--fg-dim);
    cursor: pointer;
  }
  .tab:hover {
    color: var(--fg);
  }
  .tab.on {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }
  .content {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }
</style>
