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
  import WebAuth from "./components/WebAuth.svelte";
  import type { SystemUpdate } from "./lib/transport";

  let ready = $state(false);
  let splashDone = $state(false);
  let webLocked = $state(api.inWeb());
  let unsub: (() => void) | undefined;
  let startupError = $state("");
  let updateCheckScheduled = false;

  store.transportReady = !api.inWeb();
  store.transportState = api.inWeb() ? "connecting" : "connected";

  // Retro CRT effects — three independent toggles (default on until config
  // loads). Scanlines + bloom are body classes; the roll is a rendered overlay.
  const crtScanlines = $derived(store.config?.UI?.CRTScanlines ?? true);
  const crtRoll = $derived(store.config?.UI?.CRTRoll ?? true);
  const crtBloom = $derived(store.config?.UI?.CRTBloom ?? true);
  $effect(() => {
    document.body.classList.toggle("crt-scanlines", crtScanlines);
    document.body.classList.toggle("crt-bloom", crtBloom);
  });

  function applySystem(update: SystemUpdate) {
    if (update.type === "auth-expired") {
      unsub?.();
      unsub = undefined;
      store.resetSession();
      store.transportReady = false;
      webLocked = true;
      return;
    }
    if (update.type === "config" && update.config) {
      store.installConfig(update.config);
      store.sidebarOpen = update.config.UI?.DisplayMode !== "off";
    } else if (update.type === "modes") {
      store.modeNames = update.modeNames ?? [];
      store.hasModes = store.modeNames.length > 0;
      if (update.result) {
        store.addToast(update.result.ok ? "Scripts reloaded" : "Reload failed", update.result.message ?? "");
      }
    } else if (update.type === "accounts") {
      store.accounts = update.accounts ?? [];
    } else if (update.type === "operation" && update.result) {
      store.addToast(update.result.ok ? "Operation complete" : "Operation failed", update.result.message ?? "");
    } else if (update.type === "transport" && update.transportState) {
      store.transportState = update.transportState;
      store.transportReady = update.transportState === "connected";
    }
  }

  function applyEvents(batch: import("./lib/types").WireEvent[]) {
    store.apply(batch);
    for (const event of batch) {
      if (event.kind === "notify" && event.notify) {
        api.showLocalNotification(event.notify.title, event.notify.message);
      }
    }
  }

  function scheduleUpdateCheck() {
    // The native GUI owns the upstream release-check API. The web shell has a
    // separate transport and deliberately does not ask every browser to repeat
    // a server-side update check.
    if (api.inWeb() || updateCheckScheduled) return;
    updateCheckScheduled = true;
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
  }

  async function initialize() {
    ready = false;
    startupError = "";
    if (api.inWeb()) store.transportReady = false;
    try {
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
      webLocked = false;
      ready = true;

      // Subscribe before Start() so we never miss early events.
      unsub?.();
      unsub = api.onEvents(
        applyEvents,
        (snapshot) => store.installSnapshot(snapshot),
        applySystem,
      );
      await api.start();
      scheduleUpdateCheck();
    } catch (error) {
      if (error instanceof api.WebAuthRequiredError) {
        webLocked = true;
        ready = true;
        return;
      }
      ready = true;
      startupError = error instanceof Error ? error.message : String(error);
    }
  }

  onMount(() => {
    const viewport = window.visualViewport;
    const syncViewport = () => {
      document.documentElement.style.setProperty(
        "--praetor-viewport-height",
        `${Math.round(viewport?.height ?? window.innerHeight)}px`,
      );
    };
    syncViewport();
    viewport?.addEventListener("resize", syncViewport);
    viewport?.addEventListener("scroll", syncViewport);
    window.addEventListener("resize", syncViewport);
    initialize();
    return () => {
      unsub?.();
      viewport?.removeEventListener("resize", syncViewport);
      viewport?.removeEventListener("scroll", syncViewport);
      window.removeEventListener("resize", syncViewport);
      document.documentElement.style.removeProperty("--praetor-viewport-height");
    };
  });
</script>

{#if !splashDone}
  <Splash ondismiss={() => (splashDone = true)} />
{:else if !ready}
  <div class="center"><div class="dim">Loading…</div></div>
{:else if webLocked}
  <WebAuth onunlock={initialize} />
{:else if startupError}
  <div class="center">
    <div class="startup-error">
      <div class="title">Praetor web is unavailable</div>
      <div class="dim">{startupError}</div>
      <button class="primary" onclick={initialize}>Retry</button>
    </div>
  </div>
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
  .startup-error {
    width: min(430px, calc(100vw - 32px));
    padding: 18px;
    border: 1px solid var(--red);
    background: var(--bg-panel);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .startup-error .title {
    color: var(--red);
    font-weight: 700;
  }
  .startup-error button {
    align-self: flex-start;
  }
</style>
