<script lang="ts">
  import type { Snippet } from "svelte";
  import { onMount } from "svelte";
  import { store } from "../lib/store.svelte";
  import { MENU_SECTIONS, menuParent, menuRouteLabel } from "../lib/menu";

  let modalEl: HTMLDivElement;

  let {
    title,
    wide = false,
    extraWide = false,
    back = false,
    onclose,
    onsave,
    guard,
    children,
    footer,
  }: {
    title: string;
    wide?: boolean;
    extraWide?: boolean;
    back?: boolean;
    onclose?: () => void;
    // When provided, the shell renders a "Save" button that calls onsave().
    onsave?: () => void | Promise<void>;
    // Optional veto: return false to cancel a close/back (e.g. unsaved-changes
    // prompt). Called before onclose. Absent → always allowed.
    guard?: () => boolean;
    children: Snippet;
    footer?: Snippet;
  } = $props();

  const parentRoute = $derived(back ? (menuParent(store.openModal) ?? "menu") : null);
  const parentLabel = $derived(menuRouteLabel(parentRoute));
  const inMenuShell = $derived(!!parentRoute?.startsWith("menu-"));
  const stableSettings = $derived(
    store.openModal === "menu" || !!store.openModal?.startsWith("menu-") || !!parentRoute,
  );

  // Every submenu (anything with a Back button, a Save action, or custom footer
  // actions) gets the same footer control set: [custom actions] · Close · Save.
  // Close discards — submenus buffer their edits, so closing without saving
  // restores the state from before the submenu was opened. Save commits.
  const hasFooter = $derived(back || !!onsave || !!footer);

  // The header ✕ exits the overlay entirely.
  function close() {
    if (guard && !guard()) return;
    onclose?.();
    store.openModal = null;
    store.modalEscapeTarget = null;
  }

  // The footer "Close" discards edits and returns to the parent menu for a
  // submenu (rather than exiting the overlay); standalone modals just close.
  function closeToParent() {
    if (guard && !guard()) return;
    onclose?.();
    store.openModal = parentRoute;
  }

  async function doSave() {
    await onsave?.();
  }

  function goBack() {
    if (guard && !guard()) return;
    onclose?.();
    store.openModal = parentRoute;
  }

  onMount(() => {
    // Tell the single Esc handler (in GameView) where Esc should go from this
    // modal: back to the menu for submenus, or fully closed otherwise.
    store.modalEscapeTarget = parentRoute;

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
  <div class="modal" class:wide class:extra-wide={extraWide} class:menu-shell={inMenuShell} class:stable-settings={stableSettings} bind:this={modalEl} role="dialog" aria-label={title} aria-modal="true" tabindex="-1">
    <div class="mhead">
      <div class="mhead-left">
        {#if back}
          <button class="back" onclick={goBack} title={`Back to ${parentLabel}`}>‹ {parentLabel}</button>
        {/if}
        <span class="mtitle">{title}</span>
      </div>
      <button class="x" onclick={close} aria-label="Close">✕</button>
    </div>
    <div class="modal-content" class:with-nav={inMenuShell}>
      {#if inMenuShell}
        <nav class="editor-nav" aria-label="Praetor settings">
          <button class="menu-home" onclick={() => (store.openModal = "menu")}>‹ Menu</button>
          {#each MENU_SECTIONS as section (section.id)}
            <button
              class:active={parentRoute === `menu-${section.id}`}
              onclick={() => (store.openModal = `menu-${section.id}`)}
            >{section.label}</button>
          {/each}
        </nav>
      {/if}
      <div class="mbody">
        {@render children()}
      </div>
    </div>
    {#if hasFooter}
      <div class="mfoot">
        {#if footer}{@render footer()}{/if}
        <span class="spacer"></span>
        <button onclick={closeToParent}>Close</button>
        {#if onsave}<button class="primary" onclick={doSave}>Save</button>{/if}
      </div>
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
  .modal.extra-wide {
    width: 1040px;
  }
  .modal.menu-shell { width: 1040px; }
  .modal.stable-settings {
    width: 1040px;
    height: min(740px, 86vh);
  }
  .modal.stable-settings .modal-content { flex: 1; }
  .modal.stable-settings .mbody { height: 100%; box-sizing: border-box; }
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
  .modal-content {
    min-height: 0;
    overflow: hidden;
  }
  .modal-content.with-nav {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    flex: 1;
  }
  .editor-nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    overflow-y: auto;
    border-right: 1px solid var(--border);
    background: var(--bg-elevated);
  }
  .editor-nav button {
    text-align: left;
    background: transparent;
    border-color: transparent;
  }
  .editor-nav button.active {
    color: var(--accent);
    border-color: var(--accent);
    background: var(--bg-panel);
  }
  .editor-nav .menu-home { margin-bottom: 8px; color: var(--fg-dim); }
  .mbody {
    min-width: 0;
    padding: 12px;
    overflow: auto;
  }
  .mfoot {
    padding: 8px 12px;
    border-top: 1px solid var(--border);
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  @media (max-width: 599px) {
    .backdrop {
      align-items: flex-end;
    }
    .modal,
    .modal.wide,
    .modal.extra-wide {
      width: 100%;
      max-width: 100%;
      max-height: calc(100dvh - env(safe-area-inset-top));
      border-inline: none;
      border-bottom: none;
    }
    .modal.stable-settings {
      width: 100%;
      max-width: 100%;
      height: calc(100dvh - env(safe-area-inset-top));
    }
    .modal-content.with-nav { display: block; }
    .editor-nav { display: none; }
    .mhead {
      min-height: 48px;
      padding-inline: 10px;
    }
    .back,
    .x {
      min-width: 44px;
      min-height: 44px;
    }
    .mbody {
      padding: 10px;
      overscroll-behavior: contain;
    }
    .mfoot {
      padding: 8px 10px max(8px, env(safe-area-inset-bottom));
      flex-wrap: wrap;
    }
    .mfoot button {
      min-height: 44px;
    }
  }
</style>
