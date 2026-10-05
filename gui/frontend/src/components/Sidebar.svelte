<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import MapNavigation from "./MapNavigation.svelte";
  import SidebarVitals from "./SidebarVitals.svelte";
  import SidebarTabs from "./SidebarTabs.svelte";

  const sidebarPx = $derived(store.config?.UI?.GUISidebarWidth || 260);
</script>

<div class="sidebar" class:web={api.inWeb()} style="width:{sidebarPx}px">
  <MapNavigation />
  <SidebarVitals />
  <SidebarTabs />
</div>

<style>
  .sidebar {
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
    .sidebar.web {
      display: none;
    }
  }
</style>
