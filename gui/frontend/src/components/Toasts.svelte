<script lang="ts">
  import { store } from "../lib/store.svelte";
</script>

<div class="toasts">
  {#each store.toasts as t (t.id)}
    <div class="toast">
      <div class="title">{t.title}</div>
      {#if t.message}<div class="msg">{t.message}</div>{/if}
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    bottom: 16px;
    right: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 1000;
    pointer-events: none;
  }
  .toast {
    background: var(--bg-elevated);
    border: 1px solid var(--accent-dim);
    border-left: 3px solid var(--accent);
    border-radius: 6px;
    padding: 10px 14px;
    max-width: 320px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  }
  .title {
    font-weight: 600;
    font-size: 13px;
  }
  .msg {
    font-size: 12px;
    color: var(--fg-dim);
    margin-top: 3px;
  }
</style>
