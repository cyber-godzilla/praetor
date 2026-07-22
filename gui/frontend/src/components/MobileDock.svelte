<script lang="ts">
  import { store } from "../lib/store.svelte";
  import MapNavigation from "./MapNavigation.svelte";
</script>

<div class="mobile-dock">
  <MapNavigation compact />
  <div class="tools" aria-label="Mobile tools">
    <button onclick={() => (store.openModal = "mobile-actions")}>Actions</button>
    <button onclick={() => (store.openModal = "modeselect")}>Modes</button>
    <button onclick={() => (store.openModal = "menu")}>Menu</button>
  </div>
</div>

<style>
  .mobile-dock {
    display: none;
    flex-shrink: 0;
    padding: 9px 8px max(8px, env(safe-area-inset-bottom));
    background: var(--bg-panel);
    border-top: 1px solid var(--border);
  }
  .tools {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 6px;
    margin-top: 8px;
  }
  .tools button {
    min-height: 44px;
  }
  @media (max-width: 899px) {
    .mobile-dock {
      display: block;
    }
  }
</style>
