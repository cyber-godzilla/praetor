<script lang="ts">
  import type { DisplayComponentConfig } from "../lib/types";
  import { store } from "../lib/store.svelte";
  import { integerLayoutPixels } from "../lib/layout";
  import ViewContent from "./ViewContent.svelte";

  let {
    placement,
    title,
    onsave,
  }: {
    placement: DisplayComponentConfig;
    title: string;
    onsave: () => void;
  } = $props();

  const runtime = $derived(store.viewByID(placement.ComponentID));

  function toggle() {
    placement.Collapsed = !placement.Collapsed;
    onsave();
  }

  function beginHeightResize(event: PointerEvent) {
    event.preventDefault();
    const startY = event.clientY;
    const startHeight = placement.SidebarHeight;
    const move = (next: PointerEvent) => {
      placement.SidebarHeight = integerLayoutPixels(startHeight + next.clientY - startY, 80, 800);
    };
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      onsave();
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up, { once: true });
  }
</script>

<section class="section" class:collapsed={placement.Collapsed}>
  <button class="heading" onclick={toggle} aria-expanded={!placement.Collapsed} title={placement.Collapsed ? `Expand ${title}` : `Collapse ${title}`} tabindex="-1">
    <svg class="chevron" viewBox="0 0 16 16" aria-hidden="true">
      {#if placement.Collapsed}
        <path d="M5 3 L12 8 L5 13 Z" />
      {:else}
        <path d="M3 5 L13 5 L8 12 Z" />
      {/if}
    </svg>
    {title}
    {#if runtime?.unread}<span class="unread" title="New activity">●</span>{/if}
  </button>
  {#if !placement.Collapsed}
    <div class="content" style:height={`${placement.SidebarHeight}px`}>
      <ViewContent id={placement.ComponentID} presentation="sidebar" />
    </div>
    <button class="height-handle" aria-label={`Resize ${title} height`} title="Drag to resize" onpointerdown={beginHeightResize}></button>
  {/if}
</section>

<style>
  .section { position: relative; flex: 0 0 auto; min-width: 0; border: 1px solid var(--border); background: var(--bg-panel); padding-top: 12px; }
  .section.collapsed { padding: 6px 10px; }
  .heading { position: absolute; top: 0; left: 8px; transform: translateY(-50%); display: flex; align-items: center; gap: 4px; padding: 0 5px; border: none; background: var(--bg-panel); color: var(--accent); font: inherit; font-size: 11px; line-height: 1; text-transform: uppercase; letter-spacing: 1.5px; text-align: left; white-space: nowrap; }
  .heading:hover { color: var(--fg-bright); }
  .chevron { width: 9px; height: 9px; fill: currentColor; flex-shrink: 0; }
  .unread { margin-left: auto; font-size: 8px; }
  .content { min-height: 80px; overflow: auto; display: flex; flex-direction: column; }
  .height-handle { display: block; width: 100%; height: 7px; padding: 0; border: none; border-top: 1px solid var(--border); background: transparent; cursor: ns-resize; }
  .height-handle:hover { background: var(--accent-dim); }
</style>
