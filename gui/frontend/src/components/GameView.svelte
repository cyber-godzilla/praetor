<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import StatusBar from "./StatusBar.svelte";
  import TabBar from "./TabBar.svelte";
  import OutputPane from "./OutputPane.svelte";
  import MetricsPanel from "./MetricsPanel.svelte";
  import InputLine from "./InputLine.svelte";
  import Sidebar from "./Sidebar.svelte";

  function visibleTabs() {
    return store.tabs.filter((t) => t.visible);
  }

  function cycleTab(dir: number) {
    const vis = visibleTabs();
    if (vis.length === 0) return;
    const cur = store.tabs[store.activeTab];
    let idx = vis.indexOf(cur);
    if (idx < 0) idx = 0;
    idx = (idx + dir + vis.length) % vis.length;
    store.selectTab(store.tabs.indexOf(vis[idx]));
  }

  async function quickCycleMode() {
    const modes = store.config?.UI?.QuickCycleModes ?? [];
    if (modes.length === 0) return;
    const cur = store.mode || "disable";
    const i = modes.indexOf(cur);
    const next = modes[(i + 1) % modes.length];
    try {
      await api.setMode(next, []);
    } catch (e) {
      store.addToast("Mode error", String(e));
    }
  }

  function onKeydown(e: KeyboardEvent) {
    // Let modals own the keyboard when open.
    if (store.openModal) return;

    if (e.key === "Escape") {
      e.preventDefault();
      store.openModal = "menu";
      return;
    }
    if (e.key === "Tab") {
      e.preventDefault();
      cycleTab(e.shiftKey ? -1 : 1);
      return;
    }
    if (e.altKey) {
      // Match on e.code (physical key), not e.key: on macOS, Option+<key>
      // rewrites e.key to a composed character (Option+I -> "ˆ", Option+1 ->
      // "¡"), which would break every Alt shortcut. e.code stays "KeyI"/"Digit1".
      const digit = e.code.match(/^Digit(\d)$/);
      if (e.code === "KeyS") {
        e.preventDefault();
        store.sidebarOpen = !store.sidebarOpen;
      } else if (e.code === "KeyM") {
        e.preventDefault();
        quickCycleMode();
      } else if (e.code === "KeyX") {
        e.preventDefault();
        api.setMode("disable", []).catch((err) => store.addToast("Mode error", String(err)));
      } else if (e.code === "KeyI") {
        e.preventDefault();
        store.expandAllSuppressed = !store.expandAllSuppressed;
        store.addToast(
          store.expandAllSuppressed ? "Suppressed lines shown" : "Suppressed lines hidden",
          "",
        );
      } else if (digit) {
        e.preventDefault();
        const d = parseInt(digit[1], 10);
        const n = d === 0 ? 10 : d;
        const vis = visibleTabs();
        if (n <= vis.length) store.selectTab(store.tabs.indexOf(vis[n - 1]));
      }
    }
  }

  const activeTab = $derived(store.tabs[store.activeTab]);
</script>

<svelte:window onkeydown={onKeydown} />

<div class="game">
  <StatusBar />
  <TabBar />
  <div class="body">
    <div class="main">
      {#if activeTab?.kind === "metrics"}
        <MetricsPanel />
      {:else if activeTab?.kind === "debug"}
        <OutputPane tab={activeTab} />
      {:else if activeTab}
        <OutputPane tab={activeTab} />
      {/if}
      <InputLine />
    </div>
    {#if store.sidebarOpen}
      <Sidebar />
    {/if}
  </div>
</div>

<style>
  .game {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .body {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
</style>
