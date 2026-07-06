<script lang="ts">
  import { onMount } from "svelte";
  import { store } from "./lib/store.svelte";
  import * as api from "./lib/bridge";
  import AccountSelect from "./components/AccountSelect.svelte";
  import Login from "./components/Login.svelte";
  import GameView from "./components/GameView.svelte";
  import Toasts from "./components/Toasts.svelte";
  import Modals from "./components/Modals.svelte";

  let ready = $state(false);

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
    })();
    return () => unsub?.();
  });
</script>

{#if ready}
  {#if store.screen === "account"}
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
{:else}
  <div class="center"><div class="dim">Loading…</div></div>
{/if}

<Modals />
<Toasts />

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
