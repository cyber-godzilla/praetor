<script lang="ts">
  import MapNavigation from "./MapNavigation.svelte";
  import SidebarVitals from "./SidebarVitals.svelte";
  import SidebarTabs from "./SidebarTabs.svelte";
</script>

<div class="sidebar">
  <MapNavigation />
  <SidebarVitals />
  <SidebarTabs />
</div>

<style>
  .sidebar {
    width: 260px;
    flex-shrink: 0;
    background: var(--bg-panel);
    border-left: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 12px 10px;
    overflow-y: auto;
  }
  @media (max-width: 899px) {
    .sidebar {
      display: none;
    }
  }
</style>
