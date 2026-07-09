<script lang="ts">
  import { store } from "../lib/store.svelte";
  import MenuModal from "./modals/MenuModal.svelte";
  import SettingsModal from "./modals/SettingsModal.svelte";
  import HighlightsModal from "./modals/HighlightsModal.svelte";
  import CustomTabsModal from "./modals/CustomTabsModal.svelte";
  import ActionSetsModal from "./modals/ActionSetsModal.svelte";
  import StringListModal from "./modals/StringListModal.svelte";
  import NotificationsModal from "./modals/NotificationsModal.svelte";
  import KudosModal from "./modals/KudosModal.svelte";
  import BookmarksModal from "./modals/BookmarksModal.svelte";
  import RBCalcModal from "./modals/RBCalcModal.svelte";
  import PersistentDataModal from "./modals/PersistentDataModal.svelte";
  import HelpModal from "./modals/HelpModal.svelte";
  import ModeSelectModal from "./modals/ModeSelectModal.svelte";
  import QuickCycleModal from "./modals/QuickCycleModal.svelte";
  import CRTEffectsModal from "./modals/CRTEffectsModal.svelte";
  import * as api from "../lib/bridge";

  const m = $derived(store.openModal);

  // StringListModal seeds its editor from store.config on mount, so after a save
  // we must update store.config too — otherwise reopening the modal shows the
  // stale list (the backend persists correctly, but the frontend snapshot
  // wouldn't reflect it until the next launch).
  function saveIgnoreOOC(v: string[]) {
    if (store.config?.Ignorelist) store.config.Ignorelist.OOC = v;
    return api.setIgnoreOOC(v);
  }
  function saveIgnoreThink(v: string[]) {
    if (store.config?.Ignorelist) store.config.Ignorelist.Think = v;
    return api.setIgnoreThink(v);
  }
  function saveScriptDirs(v: string[]) {
    if (store.config) store.config.Scripts = v;
    return api.setScriptDirs(v);
  }
  function saveHighPriority(v: string[]) {
    if (store.config?.Commands) store.config.Commands.HighPriority = v;
    return api.setHighPriority(v);
  }
</script>

{#if m === "menu"}
  <MenuModal />
{:else if m === "help"}
  <HelpModal />
{:else if m === "modeselect"}
  <ModeSelectModal />
{:else if m === "settings"}
  <SettingsModal />
{:else if m === "crt"}
  <CRTEffectsModal />
{:else if m === "highlights"}
  <HighlightsModal />
{:else if m === "tabs"}
  <CustomTabsModal />
{:else if m === "actionsets"}
  <ActionSetsModal />
{:else if m === "ignore-ooc"}
  <StringListModal
    title="Ignore — OOC accounts"
    hint="Lines from these OOC account names are suppressed. Press Alt+I in the game view to reveal suppressed lines."
    initial={store.config?.Ignorelist?.OOC ?? []}
    onsave={saveIgnoreOOC}
  />
{:else if m === "ignore-think"}
  <StringListModal
    title="Ignore — Think characters"
    hint="Think-channel lines from these character names are suppressed. Press Alt+I in the game view to reveal suppressed lines."
    initial={store.config?.Ignorelist?.Think ?? []}
    onsave={saveIgnoreThink}
  />
{:else if m === "scripts"}
  <StringListModal
    title="Script directories"
    hint="Directories scanned for Lua modes. Reloads on save."
    initial={store.config?.Scripts ?? []}
    onsave={saveScriptDirs}
  />
{:else if m === "priority"}
  <StringListModal
    title="High-priority commands"
    hint="Commands with an immediate precedence during script events, which will bypass other queued commands."
    initial={store.config?.Commands?.HighPriority ?? []}
    onsave={saveHighPriority}
  />
{:else if m === "quickcycle"}
  <QuickCycleModal />
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
