<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import { store } from "../lib/store.svelte";
  import { centerInBox } from "../lib/splash";

  let { ondismiss }: { ondismiss: () => void } = $props();

  let showHint = $state(false);
  let dismissed = false;

  // The version line, centered within the art box's 64-column interior so the
  // border stays aligned for any version length (a fixed 6-char slot dropped a
  // digit at two-digit patch numbers, e.g. v0.2.10 -> v0.2.1).
  const BOX_INTERIOR = 64;
  const ver = $derived(centerInBox(store.version || "dev", BOX_INTERIOR));

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
   |║|${ver}|║|
  _|_|_                                                              _|_|_
 |_____|                                                            |_____|`,
  );

  function dismiss() {
    if (dismissed) return;
    dismissed = true;
    ondismiss();
  }

  onMount(() => {
    const hint = setTimeout(() => (showHint = true), 900);
    // Auto-advance to login after 5s if the user hasn't dismissed it.
    const auto = setTimeout(dismiss, 5000);
    return () => {
      clearTimeout(hint);
      clearTimeout(auto);
    };
  });
</script>

<svelte:window onkeydown={dismiss} />

<div class="splash" onclick={dismiss} role="presentation" transition:fade={{ duration: 500 }}>
  <pre class="art">{art}</pre>
  <div class="hint" class:visible={showHint}>Press any key to continue</div>
</div>

<style>
  .splash {
    /* Fixed overlay so the fade-out reveals the login screen mounting beneath. */
    position: fixed;
    inset: 0;
    z-index: 1500;
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
