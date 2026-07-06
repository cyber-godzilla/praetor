<script lang="ts">
  import type { Snippet } from "svelte";
  import { store } from "../lib/store.svelte";

  let {
    title,
    wide = false,
    onclose,
    children,
    footer,
  }: {
    title: string;
    wide?: boolean;
    onclose?: () => void;
    children: Snippet;
    footer?: Snippet;
  } = $props();

  function close() {
    onclose?.();
    store.openModal = null;
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<!-- Backdrop click and Escape (global handler above) both dismiss the modal. -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="backdrop"
  onclick={close}
  role="presentation"
>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal" class:wide onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" tabindex="-1">
    <div class="mhead">
      <span class="mtitle">{title}</span>
      <button class="x" onclick={close} aria-label="Close">✕</button>
    </div>
    <div class="mbody">
      {@render children()}
    </div>
    {#if footer}
      <div class="mfoot">{@render footer()}</div>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 500;
  }
  .modal {
    width: 460px;
    max-width: 92vw;
    max-height: 86vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-panel);
    border: 1px solid var(--border-bright);
    border-radius: 10px;
    box-shadow: 0 12px 48px rgba(0, 0, 0, 0.5);
  }
  .modal.wide {
    width: 720px;
  }
  .mhead {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border);
  }
  .mtitle {
    font-size: 15px;
    font-weight: 600;
    color: var(--accent);
  }
  .x {
    background: none;
    border: none;
    color: var(--fg-dim);
    font-size: 14px;
    padding: 2px 6px;
  }
  .x:hover {
    color: var(--fg);
    background: var(--bg-elevated);
  }
  .mbody {
    padding: 18px;
    overflow-y: auto;
  }
  .mfoot {
    padding: 12px 18px;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
