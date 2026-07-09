<script lang="ts">
  import { tick } from "svelte";
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { clampMenuPosition } from "../lib/menu";

  let open = $state(false);
  let pos = $state({ x: 0, y: 0 });
  let selText = $state(""); // selection captured at open
  let target = $state<HTMLInputElement | HTMLTextAreaElement | null>(null); // focused input at open
  let caret = $state({ start: 0, end: 0 });
  let menuEl = $state<HTMLDivElement | undefined>(undefined);

  // Copyable text: an output/DOM selection, or a range highlighted inside the
  // captured input (window.getSelection() is empty for input-internal selections).
  const copyText = $derived(
    selText || (target && caret.end > caret.start ? target.value.slice(caret.start, caret.end) : ""),
  );

  // Only text-like inputs support selectionStart/setSelectionRange. Reading them
  // on a number/checkbox/range input throws InvalidStateError, so restrict the
  // paste target to these types (and any <textarea>).
  const SELECTABLE_INPUT_TYPES = new Set(["text", "search", "url", "tel", "password"]);
  function isTextInput(el: Element | null): el is HTMLInputElement | HTMLTextAreaElement {
    if (!el) return false;
    if (el.tagName === "TEXTAREA") return true;
    if (el.tagName === "INPUT") return SELECTABLE_INPUT_TYPES.has((el as HTMLInputElement).type);
    return false;
  }

  async function onContextMenu(e: MouseEvent) {
    e.preventDefault();
    // Snapshot NOW — clicking a menu item can move focus/caret.
    selText = window.getSelection()?.toString() ?? "";
    const active = document.activeElement;
    if (isTextInput(active)) {
      target = active;
      caret = {
        start: active.selectionStart ?? active.value.length,
        end: active.selectionEnd ?? active.value.length,
      };
    } else {
      target = null;
    }
    pos = { x: e.clientX, y: e.clientY };
    open = true;
    // Signal so the game view's Escape handler yields to us instead of opening
    // the app menu when Esc dismisses this menu.
    store.contextMenuOpen = true;
    await tick(); // let the menu render so we can measure it
    if (menuEl) {
      const r = menuEl.getBoundingClientRect();
      pos = clampMenuPosition(pos.x, pos.y, r.width, r.height, window.innerWidth, window.innerHeight);
    }
  }

  function close() {
    open = false;
    store.contextMenuOpen = false;
  }

  async function copy() {
    close();
    if (!copyText) return;
    try {
      await api.clipboardSet(copyText);
    } catch {
      // bridge already logs; nothing else to do here
    }
  }

  async function paste() {
    close();
    const el = target;
    if (!el) return;
    let text = "";
    try {
      text = await api.clipboardGet();
    } catch {
      return;
    }
    if (!text) return;
    el.value = el.value.slice(0, caret.start) + text + el.value.slice(caret.end);
    const p = caret.start + text.length;
    el.focus();
    el.setSelectionRange(p, p);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }
</script>

<svelte:window
  oncontextmenu={onContextMenu}
  onclick={close}
  onwheel={close}
  onblur={close}
  onkeydown={(e) => {
    if (e.key === "Escape") close();
  }}
/>

{#if open}
  <div class="ctxmenu" bind:this={menuEl} style="left:{pos.x}px; top:{pos.y}px" role="menu">
    <button class="item" role="menuitem" disabled={!copyText} onclick={copy}>Copy</button>
    <button class="item" role="menuitem" disabled={!target} onclick={paste}>Paste</button>
  </div>
{/if}

<style>
  .ctxmenu {
    position: fixed;
    z-index: 10000;
    min-width: 120px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-bright);
    padding: 4px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.5);
  }
  .item {
    text-align: left;
    padding: 6px 12px;
    font-size: 13px;
    background: none;
    border: none;
    color: var(--fg);
    cursor: pointer;
  }
  .item:hover:not(:disabled) {
    background: var(--bg-hover);
    color: var(--accent);
  }
  .item:disabled {
    color: var(--fg-dim);
    cursor: default;
  }
</style>
