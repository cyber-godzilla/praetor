<script lang="ts">
  import type { DisplayComponentConfig } from "../lib/types";
  import { store } from "../lib/store.svelte";
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
      placement.SidebarHeight = Math.max(80, Math.min(800, startHeight + next.clientY - startY));
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
  <button class="heading" onclick={toggle} aria-expanded={!placement.Collapsed} title={placement.Collapsed ? `Expand ${title}` : `Collapse ${title}`}>
    <span class="chevron">{placement.Collapsed ? "▸" : "▾"}</span>
    <span>{title}</span>
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
  .section { position: relative; flex: 0 0 auto; border: 1px solid var(--border); background: var(--bg-panel); min-width: 0; }
  .heading { width: 100%; display: flex; align-items: center; gap: 5px; padding: 6px 8px; border: none; border-bottom: 1px solid var(--border); background: var(--bg-elevated); color: var(--accent); font-size: 11px; text-transform: uppercase; letter-spacing: 1px; text-align: left; }
  .collapsed .heading { border-bottom: none; }
  .chevron { width: 10px; }
  .unread { margin-left: auto; font-size: 8px; }
  .content { min-height: 80px; overflow: auto; display: flex; flex-direction: column; }
  .height-handle { display: block; width: 100%; height: 7px; padding: 0; border: none; border-top: 1px solid var(--border); background: transparent; cursor: ns-resize; }
  .height-handle:hover { background: var(--accent-dim); }
</style>
