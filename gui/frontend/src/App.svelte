<script lang="ts">
  import { onMount } from "svelte";
  import { store } from "./lib/store.svelte";
  import * as api from "./lib/bridge";
  import AccountSelect from "./components/AccountSelect.svelte";
  import Login from "./components/Login.svelte";
  import GameView from "./components/GameView.svelte";
  import Splash from "./components/Splash.svelte";
  import Toasts from "./components/Toasts.svelte";
  import Modals from "./components/Modals.svelte";
  import ContextMenu from "./components/ContextMenu.svelte";

  let ready = $state(false);
  let splashDone = $state(false);

  // Retro CRT effects — three independent toggles (default on until config
  // loads). Scanlines + bloom are body classes; the roll is a rendered overlay.
  const crtScanlines = $derived(store.config?.UI?.CRTScanlines ?? true);
  const crtRoll = $derived(store.config?.UI?.CRTRoll ?? true);
  const crtBloom = $derived(store.config?.UI?.CRTBloom ?? true);
  $effect(() => {
    document.body.classList.toggle("crt-scanlines", crtScanlines);
    document.body.classList.toggle("crt-bloom", crtBloom);
  });

  onMount(() => {
    let unsub: (() => void) | undefined;
    (async () => {
      const init = await api.getInitState();
      store.version = init.version;
      store.debug = init.debug;
      store.accounts = init.accounts ?? [];
      store.config = init.config;
      store.modeNames = init.modeNames ?? [];
      store.hasModes = init.hasModes;
      store.sidebarOpen = init.config?.UI?.DisplayMode !== "off";
      store.rebuildTabs(init.config?.UI?.CustomTabs);
      store.screen = store.accounts.length > 0 ? "account" : "login";
      ready = true;

      // Subscribe before Start() so we never miss early events.
      unsub = api.onEvents((batch) => store.apply(batch));
      await api.start();

      // Quietly check for a newer release (config-gated on the Go side, which
      // also swallows network failures). Delayed so the toast lands after the
      // splash instead of underneath it.
      setTimeout(async () => {
        try {
          const u = await api.checkForUpdate();
          if (u.available) {
            store.addToast(
              "Update available",
              `Praetor ${u.latest} is out (you have ${u.current}). Grab it from GitHub releases or your package manager.`,
              15000,
            );
          }
        } catch {
          // Never surface update-check failures at startup.
        }
      }, 2500);
    })();
    return () => unsub?.();
  });
</script>

{#if !splashDone}
  <Splash ondismiss={() => (splashDone = true)} />
{:else if !ready}
  <div class="center"><div class="dim">Loading…</div></div>
{:else if store.screen === "account"}
  <AccountSelect />
{:else if store.screen === "login"}
  <Login />
{:else if store.screen === "connecting"}
  <div class="center">
    <div class="spinner-box">
      <div class="dim">Connecting to The Eternal City…</div>
    </div>
  </div>
{:else if store.screen === "game"}
  <GameView />
{/if}

<Modals />
<Toasts />
<ContextMenu />

{#if crtRoll}
  <div class="crt-roll"></div>
{/if}

<style>
  .center {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .spinner-box {
    text-align: center;
  }
</style>
