<script lang="ts">
  import type { DisplaySidebarConfig } from "../lib/types";
  import { store } from "../lib/store.svelte";
  import { COLLAPSED_SIDEBAR_WIDTH, componentName, integerLayoutPixels, sidebarComponentIDs } from "../lib/layout";
  import SidebarSection from "./SidebarSection.svelte";

  let {
    sidebar,
    drawer = false,
    collapsed = false,
    oncollapse,
    onexpand,
    onsave,
  }: {
    sidebar: DisplaySidebarConfig;
    drawer?: boolean;
    collapsed?: boolean;
    oncollapse?: () => void;
    onexpand?: () => void;
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
      sidebar.Width = integerLayoutPixels(startWidth + direction * (next.clientX - startX), 180, 600);
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
  class:collapsed
  class:left={sidebar.Side === "left"}
  class:right={sidebar.Side === "right"}
  aria-label={sidebar.Name}
  style:width={drawer ? undefined : `${collapsed ? COLLAPSED_SIDEBAR_WIDTH : sidebar.Width}px`}
>
  {#if collapsed && !drawer}
    <button class="sidebar-toggle expand" aria-label={`Expand ${sidebar.Name}`} aria-expanded="false" title={`Expand ${sidebar.Name}`} onclick={onexpand}>
      <svg viewBox="0 0 16 16" aria-hidden="true">
        {#if sidebar.Side === "left"}<path d="M5 3 L12 8 L5 13 Z" />{:else}<path d="M11 3 L4 8 L11 13 Z" />{/if}
      </svg>
    </button>
    <div class="collapsed-title"><span>{sidebar.Name}</span></div>
  {:else}
    {#if !drawer}
      <div class="sidebar-header">
        <span>{sidebar.Name}</span>
        <button class="sidebar-toggle collapse" aria-label={`Collapse ${sidebar.Name}`} aria-expanded="true" title={`Collapse ${sidebar.Name}`} onclick={oncollapse}>
          <svg viewBox="0 0 16 16" aria-hidden="true">
            {#if sidebar.Side === "left"}<path d="M11 3 L4 8 L11 13 Z" />{:else}<path d="M5 3 L12 8 L5 13 Z" />{/if}
          </svg>
        </button>
      </div>
      <button class="width-handle" aria-label={`Resize ${sidebar.Name}`} title="Drag to resize; double-click to reset" onpointerdown={beginWidthResize} ondblclick={resetWidth}></button>
    {/if}
    <div class="scroll">
      {#each placements as placement (placement.ComponentID)}
        <SidebarSection {placement} title={componentName(placement.ComponentID, custom)} {onsave} />
      {/each}
    </div>
  {/if}
</aside>

<style>
  .column { position: relative; flex: 0 0 auto; min-width: 180px; max-width: 600px; min-height: 0; display: flex; flex-direction: column; background: var(--bg-panel); }
  .column.left { border-right: 1px solid var(--border); }
  .column.right { border-left: 1px solid var(--border); }
  .column.collapsed { min-width: 30px; max-width: 30px; }
  .column.drawer { width: 100%; min-width: 0; max-width: none; border: none; }
  .sidebar-header { flex: 0 0 auto; height: 28px; display: flex; align-items: center; justify-content: space-between; padding-left: 8px; border-bottom: 1px solid var(--border); color: var(--accent); font-size: 11px; line-height: 1; text-transform: uppercase; letter-spacing: 1.5px; }
  .sidebar-toggle { flex: 0 0 auto; width: 28px; height: 28px; display: grid; place-items: center; padding: 6px; border: 0; background: transparent; color: var(--accent); }
  .sidebar-toggle:hover { background: var(--bg-elevated); color: var(--fg-bright); }
  .sidebar-toggle svg { width: 12px; height: 12px; fill: currentColor; }
  .collapsed-title { flex: 1; min-height: 0; display: grid; place-items: center; padding: 6px 4px; color: var(--accent); overflow: hidden; }
  .collapsed-title span { display: inline-block; writing-mode: vertical-rl; transform: rotate(180deg); font-size: 11px; line-height: 1; text-transform: uppercase; letter-spacing: 1.5px; white-space: nowrap; }
  .scroll { flex: 1; min-height: 0; overflow-y: auto; overflow-x: hidden; display: flex; flex-direction: column; gap: 10px; padding: 10px 8px; box-sizing: border-box; }
  .width-handle { position: absolute; z-index: 3; top: 0; bottom: 0; width: 7px; padding: 0; border: none; background: transparent; cursor: ew-resize; }
  .left .width-handle { right: -4px; }
  .right .width-handle { left: -4px; }
  .width-handle:hover { background: var(--accent-dim); }
</style>
