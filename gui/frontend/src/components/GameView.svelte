<script lang="ts">
  import { onMount } from "svelte";
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";
  import { numpadCommand } from "../lib/numpad";
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
    // Esc is the single source of truth for open/close, handled in every state
    // so it stays consistent through opening and closing menus: it closes any
    // open modal, otherwise opens the menu.
    if (e.key === "Escape") {
      // The custom right-click menu owns Escape while it's open — yield so
      // dismissing it doesn't also pop the app menu.
      if (store.contextMenuOpen) return;
      e.preventDefault();
      // From a submenu, Esc goes back to the menu (modalEscapeTarget = "menu");
      // from the menu or a standalone modal it closes; with nothing open it
      // opens the menu.
      store.openModal = store.openModal ? store.modalEscapeTarget : "menu";
      return;
    }
    // All other shortcuts are inert while a modal owns the keyboard.
    if (store.openModal) return;

    // Numpad navigation: NumLock OFF drives movement (NumLock ON types digits).
    // NumLock state is read from e.key inside numpadCommand — WebKitGTK doesn't
    // report it via getModifierState. stopPropagation so the NumLock-off arrow
    // aliases (Numpad8 => ArrowUp) never reach the input's history handler.
    const npCmd = numpadCommand(e.code, e.key, store.config?.UI?.NumpadNavigation ?? "numlock");
    if (npCmd) {
      e.preventDefault();
      e.stopPropagation();
      api.send(npCmd);
      return;
    }

    // Match on e.code (physical key), not e.key: on X11/WebKitGTK, Shift+Tab
    // emits the ISO_Left_Tab keysym, so e.key is NOT "Tab" for the reverse case
    // — an e.key check catches forward Tab but silently misses Shift+Tab.
    // e.code stays "Tab" regardless of the Shift modifier.
    if (e.code === "Tab") {
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

  // Register keydown in the CAPTURE phase. A bubble-phase handler's
  // preventDefault runs too late in WebKitGTK to stop native Tab focus
  // traversal, so Shift+Tab moved focus through the UI's many buttons instead
  // of cycling tabs. Capturing lets preventDefault win before traversal.
  onMount(() => {
    window.addEventListener("keydown", onKeydown, true);
    return () => window.removeEventListener("keydown", onKeydown, true);
  });
</script>

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
