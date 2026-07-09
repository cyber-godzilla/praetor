<script lang="ts">
  import { tick } from "svelte";
  import * as api from "../lib/bridge";
  import { clampMenuPosition } from "../lib/menu";

  let open = $state(false);
  let pos = $state({ x: 0, y: 0 });
  let selText = $state(""); // selection captured at open
  let target = $state<HTMLInputElement | HTMLTextAreaElement | null>(null); // focused input at open
  let caret = $state({ start: 0, end: 0 });
  let menuEl = $state<HTMLDivElement | undefined>(undefined);

  function isTextInput(el: Element | null): el is HTMLInputElement | HTMLTextAreaElement {
    return !!el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA");
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
    await tick(); // let the menu render so we can measure it
    if (menuEl) {
      const r = menuEl.getBoundingClientRect();
      pos = clampMenuPosition(pos.x, pos.y, r.width, r.height, window.innerWidth, window.innerHeight);
    }
  }

  function close() {
    open = false;
  }

  async function copy() {
    close();
    if (selText) await api.clipboardSet(selText);
  }

  async function paste() {
    close();
    const el = target;
    if (!el) return;
    const text = await api.clipboardGet();
    if (!text) return;
    el.value = el.value.slice(0, caret.start) + text + el.value.slice(caret.end);
    const p = caret.start + text.length;
    el.focus();
    el.setSelectionRange(p, p);
    // Notify Svelte's bind:value (execCommand-free path).
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
    <button class="item" role="menuitem" disabled={!selText} onclick={copy}>Copy</button>
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
