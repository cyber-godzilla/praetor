<script lang="ts">
  import type { Snippet } from "svelte";

  // A titled box-drawing frame: a 1px border with the title breaking through
  // the top edge, mimicking a TUI panel (┌─ TITLE ──────┐). The title's
  // background must match the surrounding surface so it "cuts" the border;
  // override via the `surface` prop (defaults to the sidebar panel color).
  let {
    title,
    surface = "var(--bg-panel)",
    children,
  }: { title: string; surface?: string; children: Snippet } = $props();
</script>

<div class="frame">
  <span class="frame-title" style="background:{surface}">{title}</span>
  {@render children()}
</div>

<style>
  .frame {
    position: relative;
    border: 1px solid var(--border);
    padding: 12px 10px 10px;
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
</style>
