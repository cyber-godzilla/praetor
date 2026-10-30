<script lang="ts">
  import { store } from "../lib/store.svelte";
  import MenuModal from "./modals/MenuModal.svelte";
  import MenuSectionModal from "./modals/MenuSectionModal.svelte";
  import SettingsModal from "./modals/SettingsModal.svelte";
  import VariablesModal from "./modals/VariablesModal.svelte";
  import CharacterProfilesModal from "./modals/CharacterProfilesModal.svelte";
  import HighlightsModal from "./modals/HighlightsModal.svelte";
  import SpeechColorsModal from "./modals/SpeechColorsModal.svelte";
  import LayoutModal from "./modals/LayoutModal.svelte";
  import ViewLibraryModal from "./modals/ViewLibraryModal.svelte";
  import ActionSetsModal from "./modals/ActionSetsModal.svelte";
  import AddActionSetModal from "./modals/AddActionSetModal.svelte";
  import AddActionModal from "./modals/AddActionModal.svelte";
  import StringListModal from "./modals/StringListModal.svelte";
  import NotificationsModal from "./modals/NotificationsModal.svelte";
  import KudosModal from "./modals/KudosModal.svelte";
  import BookmarksModal from "./modals/BookmarksModal.svelte";
  import RBCalcModal from "./modals/RBCalcModal.svelte";
  import PersistentDataModal from "./modals/PersistentDataModal.svelte";
  import NotesModal from "./modals/NotesModal.svelte";
  import SendFileModal from "./modals/SendFileModal.svelte";
  import PlayScriptModal from "./modals/PlayScriptModal.svelte";
  import HelpModal from "./modals/HelpModal.svelte";
  import ModeSelectModal from "./modals/ModeSelectModal.svelte";
  import QuickCycleModal from "./modals/QuickCycleModal.svelte";
  import PriorityCommandsModal from "./modals/PriorityCommandsModal.svelte";
  import ScriptManagerModal from "./modals/ScriptManagerModal.svelte";
  import CRTEffectsModal from "./modals/CRTEffectsModal.svelte";
  import MobileActionsModal from "./modals/MobileActionsModal.svelte";
  import NewUserModal from "./modals/NewUserModal.svelte";
  import * as api from "../lib/bridge";

  const m = $derived(store.openModal);

  // StringListModal seeds its editor from store.config on mount, so after a save
  // we must update store.config too — otherwise reopening the modal shows the
  // stale list (the backend persists correctly, but the frontend snapshot
  // wouldn't reflect it until the next launch).
  async function saveIgnoreOOC(v: string[]) {
    await api.setIgnoreOOC(v);
    if (store.config?.Ignorelist) store.config.Ignorelist.OOC = v;
  }
  async function saveIgnoreThink(v: string[]) {
    await api.setIgnoreThink(v);
    if (store.config?.Ignorelist) store.config.Ignorelist.Think = v;
  }
</script>

{#if m === "menu"}
  <MenuModal />
{:else if m?.startsWith("menu-")}
  <MenuSectionModal sectionId={m.slice(5)} />
{:else if m === "new-user"}
  <NewUserModal />
{:else if m === "help"}
  <HelpModal />
{:else if m === "modeselect"}
  <ModeSelectModal />
{:else if m === "settings"}
  <SettingsModal />
{:else if m === "settings-display"}
  <SettingsModal section="display" />
{:else if m === "settings-input"}
  <SettingsModal section="input" />
{:else if m === "settings-logs"}
  <SettingsModal section="logs" />
{:else if m === "settings-application"}
  <SettingsModal section="application" />
{:else if m === "variables"}
  <VariablesModal />
{:else if m === "character-profiles"}
  <CharacterProfilesModal />
{:else if m === "crt"}
  <CRTEffectsModal />
{:else if m === "highlights"}
  <HighlightsModal />
{:else if m === "speech-colors"}
  <SpeechColorsModal />
{:else if m === "layout"}
  <LayoutModal />
{:else if m === "view-library"}
  <ViewLibraryModal />
{:else if m === "actionsets"}
  <ActionSetsModal />
{:else if m === "actionset-add"}
  <AddActionSetModal />
{:else if m === "action-add"}
  <AddActionModal />
{:else if m === "ignore-ooc"}
  <StringListModal
    title="Ignore — OOC accounts"
    hint="Lines from these OOC account names are suppressed. Press Alt+I in the game view to reveal suppressed lines."
    initial={store.config?.Ignorelist?.OOC ?? []}
    onsave={saveIgnoreOOC}
    variablePatterns
  />
{:else if m === "ignore-think"}
  <StringListModal
    title="Ignore — Think characters"
    hint="Think-channel lines from these character names are suppressed. Press Alt+I in the game view to reveal suppressed lines."
    initial={store.config?.Ignorelist?.Think ?? []}
    onsave={saveIgnoreThink}
    variablePatterns
  />
{:else if m === "scripts"}
  <ScriptManagerModal />
{:else if m === "priority"}
  <PriorityCommandsModal />
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
{:else if m === "notes"}
  <NotesModal />
{:else if m === "sendfile"}
  <SendFileModal />
{:else if m === "playscript"}
  <PlayScriptModal />
{:else if m === "mobile-actions"}
  <MobileActionsModal />
{/if}
