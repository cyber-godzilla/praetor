<script lang="ts">
  import type { DisplaySidebarConfig } from "../lib/types";
  import { store } from "../lib/store.svelte";
  import { componentName, sidebarComponentIDs } from "../lib/layout";
  import SidebarSection from "./SidebarSection.svelte";

  let {
    sidebar,
    drawer = false,
    onsave,
  }: {
    sidebar: DisplaySidebarConfig;
    drawer?: boolean;
    onsave: () => void;
  } = $props();

  const custom = $derived(store.characterProfile()?.CustomViews ?? []);
  const visibleIDs = $derived(sidebarComponentIDs(sidebar, custom, store.debug));
  const placements = $derived(sidebar.Components.filter((placement) => visibleIDs.includes(placement.ComponentID)));

  function beginWidthResize(event: PointerEvent) {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = sidebar.Width;
    const direction = sidebar.Side === "left" ? 1 : -1;
    const move = (next: PointerEvent) => {
      sidebar.Width = Math.max(180, Math.min(600, startWidth + direction * (next.clientX - startX)));
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      onsave();
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up, { once: true });
  }

  function resetWidth() {
    sidebar.Width = 260;
    onsave();
  }
</script>

<aside
  class="column"
  class:drawer
  class:left={sidebar.Side === "left"}
  class:right={sidebar.Side === "right"}
  aria-label={sidebar.Name}
  style:width={drawer ? undefined : `${sidebar.Width}px`}
>
  {#if !drawer}
    <button class="width-handle" aria-label={`Resize ${sidebar.Name}`} title="Drag to resize; double-click to reset" onpointerdown={beginWidthResize} ondblclick={resetWidth}></button>
  {/if}
  <div class="scroll">
    {#each placements as placement (placement.ComponentID)}
      <SidebarSection {placement} title={componentName(placement.ComponentID, custom)} {onsave} />
    {/each}
  </div>
</aside>

<style>
  .column { position: relative; flex: 0 0 auto; min-width: 180px; max-width: 600px; min-height: 0; background: var(--bg-panel); }
  .column.left { border-right: 1px solid var(--border); }
  .column.right { border-left: 1px solid var(--border); }
  .column.drawer { width: 100%; min-width: 0; max-width: none; border: none; }
  .scroll { height: 100%; overflow-y: auto; overflow-x: hidden; display: flex; flex-direction: column; gap: 10px; padding: 10px 8px; box-sizing: border-box; }
  .width-handle { position: absolute; z-index: 3; top: 0; bottom: 0; width: 7px; padding: 0; border: none; background: transparent; cursor: ew-resize; }
  .left .width-handle { right: -4px; }
  .right .width-handle { left: -4px; }
  .width-handle:hover { background: var(--accent-dim); }
</style>
