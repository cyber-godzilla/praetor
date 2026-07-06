<script lang="ts">
  import { onMount } from "svelte";
  import { store } from "../lib/store.svelte";

  let { ondismiss }: { ondismiss: () => void } = $props();

  let showHint = $state(false);

  // Pad the version to 6 chars to preserve the art's alignment (as the TUI does).
  const ver = $derived.by(() => {
    let v = store.version || "dev";
    if (v.length < 6) v = v + " ".repeat(6 - v.length);
    else if (v.length > 6) v = v.slice(0, 6);
    return v;
  });

  // The PRAETOR art from internal/ui/splash.go (%s -> version).
  const art = $derived(
    `  _._._                                                              _._._
 )_   _(                                                            )_   _(
   |_|                                                                |_|
   |║|  ██████╗ ██████╗  █████╗ ███████╗████████╗ ██████╗ ██████╗     |║|
   |║|  ██╔══██╗██╔══██╗██╔══██╗██╔════╝╚══██╔══╝██╔═══██╗██╔══██╗    |║|
   |║|  ██████╔╝██████╔╝███████║█████╗     ██║   ██║   ██║██████╔╝    |║|
   |║|  ██╔═══╝ ██╔══██╗██╔══██║██╔══╝     ██║   ██║   ██║██╔══██╗    |║|
   |║|  ██║     ██║  ██║██║  ██║███████╗   ██║   ╚██████╔╝██║  ██║    |║|
   |║|  ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝    |║|
   |║|                           ═══ ✦ ═══                            |║|
   |║|                            ${ver}                              |║|
  _|_|_                                                              _|_|_
 |_____|                                                            |_____|`,
  );

  function dismiss() {
    ondismiss();
  }

  onMount(() => {
    const t = setTimeout(() => (showHint = true), 900);
    return () => clearTimeout(t);
  });
</script>

<svelte:window onkeydown={dismiss} />

<div class="splash" onclick={dismiss} role="presentation">
  <pre class="art">{art}</pre>
  <div class="hint" class:visible={showHint}>Press any key to continue</div>
</div>

<style>
  .splash {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: var(--bg);
    cursor: pointer;
    gap: 28px;
  }
  .art {
    color: var(--accent);
    font-family: var(--mono);
    font-size: 13px;
    line-height: 1.15;
    margin: 0;
    white-space: pre;
    text-shadow: 0 0 18px rgba(232, 168, 56, 0.25);
  }
  .hint {
    color: var(--fg-dim);
    font-size: 13px;
    opacity: 0;
    transition: opacity 0.5s ease;
  }
  .hint.visible {
    opacity: 1;
  }
</style>
