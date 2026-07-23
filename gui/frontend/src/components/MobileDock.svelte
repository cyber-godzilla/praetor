<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import MapNavigation from "./MapNavigation.svelte";

  // These preferences are intentionally browser-only. A narrow native Wails
  // window keeps its existing dock behavior even though both shells share the
  // same serialized config shape.
  const showNavigation = $derived(
    !api.inWeb() ||
      !(store.config?.UI?.MobileHideNavigationOnInput && store.mobileCommandInputActive),
  );
  const showToolbar = $derived(
    !api.inWeb() || (store.config?.UI?.MobileShowToolbar ?? true),
  );

  function openMobileModal(name: string) {
    // Clear the focus-derived navigation suppression only after the tool click
    // has been dispatched, so restoring the map cannot move the tap target out
    // from under the pointer.
    store.mobileCommandInputActive = false;
    store.openModal = name;
  }
</script>

{#if showNavigation || showToolbar}
  <div class="mobile-dock" data-mobile-dock>
    {#if showNavigation}
      <MapNavigation compact />
    {/if}
    {#if showToolbar}
      <div class="tools" class:standalone={!showNavigation} aria-label="Mobile tools" data-mobile-tools>
        <button onclick={() => openMobileModal("mobile-actions")}>Actions</button>
        <button onclick={() => openMobileModal("modeselect")}>Modes</button>
        <button onclick={() => openMobileModal("menu")}>Menu</button>
      </div>
    {/if}
  </div>
{/if}

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
  .tools.standalone {
    margin-top: 0;
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
