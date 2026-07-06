<script lang="ts">
  import { store } from "../lib/store.svelte";
  import MenuModal from "./modals/MenuModal.svelte";
  import SettingsModal from "./modals/SettingsModal.svelte";
  import HighlightsModal from "./modals/HighlightsModal.svelte";
  import CustomTabsModal from "./modals/CustomTabsModal.svelte";
  import StringListModal from "./modals/StringListModal.svelte";
  import NotificationsModal from "./modals/NotificationsModal.svelte";
  import KudosModal from "./modals/KudosModal.svelte";
  import BookmarksModal from "./modals/BookmarksModal.svelte";
  import RBCalcModal from "./modals/RBCalcModal.svelte";
  import PersistentDataModal from "./modals/PersistentDataModal.svelte";
  import HelpModal from "./modals/HelpModal.svelte";
  import ModeSelectModal from "./modals/ModeSelectModal.svelte";
  import * as api from "../lib/bridge";

  const m = $derived(store.openModal);
</script>

{#if m === "menu"}
  <MenuModal />
{:else if m === "help"}
  <HelpModal />
{:else if m === "modeselect"}
  <ModeSelectModal />
{:else if m === "settings"}
  <SettingsModal />
{:else if m === "highlights"}
  <HighlightsModal />
{:else if m === "tabs"}
  <CustomTabsModal />
{:else if m === "ignore-ooc"}
  <StringListModal
    title="Ignore — OOC accounts"
    hint="Lines from these OOC account names are suppressed."
    initial={store.config?.Ignorelist?.OOC ?? []}
    onsave={(v) => api.setIgnoreOOC(v)}
  />
{:else if m === "ignore-think"}
  <StringListModal
    title="Ignore — Think characters"
    hint="Think-channel lines from these character names are suppressed."
    initial={store.config?.Ignorelist?.Think ?? []}
    onsave={(v) => api.setIgnoreThink(v)}
  />
{:else if m === "scripts"}
  <StringListModal
    title="Script directories"
    hint="Directories scanned for Lua modes. Reloads on save."
    initial={store.config?.Scripts ?? []}
    onsave={(v) => api.setScriptDirs(v)}
  />
{:else if m === "priority"}
  <StringListModal
    title="High-priority commands"
    hint="Commands that jump the queue."
    initial={store.config?.Commands?.HighPriority ?? []}
    onsave={(v) => api.setHighPriority(v)}
  />
{:else if m === "quickcycle"}
  <StringListModal
    title="Quick-cycle modes (Alt+M)"
    hint="Mode names cycled by Alt+M. Available: {(store.modeNames ?? []).join(', ') || 'none loaded'}"
    initial={store.config?.UI?.QuickCycleModes ?? []}
    onsave={(v) => api.setQuickCycleModes(v)}
  />
{:else if m === "notifications"}
  <NotificationsModal />
{:else if m === "kudos" || m === "kudos-login"}
  <KudosModal loginPrompt={m === "kudos-login"} />
{:else if m === "wiki"}
  <BookmarksModal title="Wiki bookmarks" kind="wiki" />
{:else if m === "maps"}
  <BookmarksModal title="Map bookmarks" kind="maps" />
{:else if m === "calc"}
  <RBCalcModal />
{:else if m === "persistent"}
  <PersistentDataModal />
{/if}
