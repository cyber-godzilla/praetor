<script lang="ts">
  import type { Snippet } from "svelte";

  // A titled box-drawing frame: a 1px border with the title breaking through
  // the top edge, mimicking a TUI panel (┌─ TITLE ──────┐). The title's
  // background must match the surrounding surface so it "cuts" the border;
  // override via the `surface` prop (defaults to the sidebar panel color).
  //
  // When `collapsible`, the title becomes a click target with a Roman-style
  // chevron and the body hides while `collapsed` is true. State is bindable so
  // the parent (e.g. the store) can own and persist it across remounts.
  let {
    title,
    surface = "var(--bg-panel)",
    collapsible = false,
    collapsed = $bindable(false),
    children,
  }: {
    title: string;
    surface?: string;
    collapsible?: boolean;
    collapsed?: boolean;
    children: Snippet;
  } = $props();

  function toggle() {
    collapsed = !collapsed;
  }
</script>

<div class="frame" class:collapsed={collapsible && collapsed}>
  {#if collapsible}
    <button
      class="frame-title frame-toggle"
      style="background:{surface}"
      onclick={toggle}
      aria-expanded={!collapsed}
      title={collapsed ? `Expand ${title}` : `Collapse ${title}`}
      tabindex="-1"
    >
      <svg class="chevron" viewBox="0 0 16 16" aria-hidden="true">
        {#if collapsed}
          <path d="M5 3 L12 8 L5 13 Z" />
        {:else}
          <path d="M3 5 L13 5 L8 12 Z" />
        {/if}
      </svg>
      {title}
    </button>
  {:else}
    <span class="frame-title" style="background:{surface}">{title}</span>
  {/if}
  {#if !collapsible || !collapsed}
    {@render children()}
  {/if}
</div>

<style>
  .frame {
    position: relative;
    border: 1px solid var(--border);
    padding: 12px 10px 10px;
  }
  /* Collapsed: no body, so shrink to a slim labeled bar. */
  .frame.collapsed {
    padding: 6px 10px;
  }
  .frame-title {
    position: absolute;
    top: 0;
    left: 8px;
    transform: translateY(-50%);
    padding: 0 5px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1.5px;
    color: var(--accent);
    white-space: nowrap;
  }
  /* Reset button chrome so the toggle looks identical to the plain title,
     just clickable with a chevron. */
  .frame-toggle {
    display: flex;
    align-items: center;
    gap: 4px;
    border: none;
    font: inherit;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1.5px;
    cursor: pointer;
    line-height: 1;
  }
  .frame-toggle:hover {
    color: var(--fg-bright);
  }
  .chevron {
    width: 9px;
    height: 9px;
    fill: currentColor;
    flex-shrink: 0;
  }
</style>
