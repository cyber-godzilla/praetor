<script lang="ts">
  import type { Snippet } from "svelte";
  import { onMount } from "svelte";
  import { store } from "../lib/store.svelte";

  let modalEl: HTMLDivElement;

  let {
    title,
    wide = false,
    back = false,
    onclose,
    children,
    footer,
  }: {
    title: string;
    wide?: boolean;
    back?: boolean;
    onclose?: () => void;
    children: Snippet;
    footer?: Snippet;
  } = $props();

  function close() {
    onclose?.();
    store.openModal = null;
  }

  function goBack() {
    onclose?.();
    store.openModal = "menu";
  }

  onMount(() => {
    // Tell the single Esc handler (in GameView) where Esc should go from this
    // modal: back to the menu for submenus, or fully closed otherwise.
    store.modalEscapeTarget = back ? "menu" : null;

    // Move focus into the modal on open so keystrokes don't leak into the (still
    // focused) game input behind the backdrop. Prefer a text field; otherwise
    // focus the dialog container itself (tabindex=-1).
    const field = modalEl?.querySelector<HTMLElement>(
      '.mbody input[type="text"], .mbody input[type="password"], .mbody input[type="number"], .mbody textarea',
    );
    (field ?? modalEl)?.focus();
  });
</script>

<!-- The backdrop only dims; it does NOT close on click. Closing is explicit:
     the ✕ button, the Back button, or Esc (handled once in GameView). This
     prevents stray outside clicks from dismissing a submenu mid-interaction. -->
<div class="backdrop" role="presentation">
  <div class="modal" class:wide bind:this={modalEl} role="dialog" aria-modal="true" tabindex="-1">
    <div class="mhead">
      <div class="mhead-left">
        {#if back}
          <button class="back" onclick={goBack} title="Back to menu">‹ Menu</button>
        {/if}
        <span class="mtitle">{title}</span>
      </div>
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
    border: 1px solid var(--accent);
  }
  .modal.wide {
    width: 720px;
  }
  .mhead {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
  }
  .mhead-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .back {
    font-size: 12px;
    padding: 2px 8px;
    color: var(--fg-dim);
  }
  .back:hover {
    color: var(--accent);
    border-color: var(--accent);
  }
  /* Title in the TUI style: uppercase, orange, letter-spaced. */
  .mtitle {
    font-size: 13px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1.5px;
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
    padding: 12px;
    overflow-y: auto;
  }
  .mfoot {
    padding: 8px 12px;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
