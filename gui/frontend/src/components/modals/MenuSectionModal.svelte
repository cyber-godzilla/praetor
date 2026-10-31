<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import {
    MENU_SECTIONS,
    groupMenuEntriesByScope,
    menuScopeLabel,
    menuSection,
    type MenuEntry,
  } from "../../lib/menu";
  import * as api from "../../lib/bridge";

  let { sectionId }: { sectionId: string } = $props();

  const section = $derived(menuSection(sectionId));
  const profiles = $derived(store.config?.CharacterProfiles?.Profiles ?? []);
  const activeProfile = $derived(store.characterProfile());
  const scopeGroups = $derived(section ? groupMenuEntriesByScope(section, entryScope) : []);

  function open(route: string) {
    store.openModal = route;
  }

  async function reloadScripts() {
    try {
      await api.reloadScripts();
      [store.modeNames, store.modeSpecs] = await Promise.all([
        api.modeNames(),
        api.modeSpecs(),
      ]);
      store.addToast("Scripts reloaded", "");
    } catch (error) {
      store.addToast("Reload failed", String(error));
    }
  }

  async function activate(entry: MenuEntry) {
    if (entry.route) open(entry.route);
    else if (entry.action === "reload-scripts") await reloadScripts();
  }

  function entryScope(entry: MenuEntry) {
    if (entry.route === "settings-logs" && !api.inWeb()) return "device";
    return entry.scope ?? section?.scope ?? "global";
  }

  async function switchProfile(id: string) {
    if (id === store.activeCharacterProfileID) return;
    try {
      await api.switchCharacterProfile(id, false);
    } catch (error) {
      const message = String(error);
      if (message.includes("Lua mode") && confirm(`${message}\n\nStop the mode and switch characters?`)) {
        try {
          await api.switchCharacterProfile(id, true);
          return;
        } catch (retry) {
          store.addToast("Character switch blocked", String(retry));
          return;
        }
      }
      store.addToast("Character switch blocked", message);
    }
  }

  async function logout() {
    if (
      api.inWeb() &&
      !window.confirm("Disconnect the shared TEC game session for every connected browser?")
    ) return;
    store.openModal = null;
    try {
      await api.disconnect();
    } catch (error) {
      store.addToast("Disconnect failed", error instanceof Error ? error.message : String(error));
    }
  }
</script>

<Modal title={`Praetor / ${section?.label ?? "Menu"}`} extraWide back>
  <div class="settings-shell">
    <nav aria-label="Praetor settings">
      <button class="menu-home" onclick={() => open("menu")}>‹ Menu</button>
      {#each MENU_SECTIONS as item (item.id)}
        <button class:active={sectionId === item.id} onclick={() => open(`menu-${item.id}`)}>
          {item.label}
        </button>
      {/each}
      <div class="nav-spacer"></div>
      {#if api.inWeb()}
        <button onclick={logout}>Disconnect shared game</button>
        <button onclick={() => void api.quit()}>Sign out of web UI</button>
      {:else}
        <button onclick={logout}>Logout</button>
        <button onclick={() => void api.quit()}>Quit Praetor</button>
      {/if}
    </nav>

    <main>
      <header class="page-head">
        <div>
          <h2>{section?.label}</h2>
          <p>{section?.description}</p>
        </div>
        {#if section}
          <span class="scope-label">{menuScopeLabel(section.scope)}</span>
        {/if}
      </header>

      {#if sectionId === "character"}
        <section class="block">
          <h3>Switch character</h3>
          <div class="profile-grid">
            {#each profiles as profile (profile.ID)}
              <button
                class:chosen={profile.ID === store.activeCharacterProfileID}
                aria-pressed={profile.ID === store.activeCharacterProfileID}
                onclick={() => switchProfile(profile.ID)}
              >
                <span><strong>{profile.Name}</strong><small>{profile.Account || (profile.ID === store.config?.CharacterProfiles?.DefaultID ? "Global template" : "No account")}</small></span>
                {#if profile.ID === store.activeCharacterProfileID}<b>Active</b>{/if}
              </button>
            {/each}
          </div>
        </section>
      {/if}

      {#each scopeGroups as group (group.scope)}
        <section class="block scope-group">
          <h3>{menuScopeLabel(group.scope)}</h3>
          {#if group.scope === "character" && activeProfile}
            <p class="scope-copy">These settings currently apply to <strong>{activeProfile.Name}</strong>.</p>
          {/if}
          <div class="entry-list">
            {#each group.entries as entry (entry.label)}
              <button onclick={() => activate(entry)}>
                <span><strong>{entry.label}</strong><small>{entry.description}</small></span>
                <span class="entry-end"><i>{entry.route ? "›" : "↻"}</i></span>
              </button>
            {/each}
          </div>
        </section>
      {/each}

      {#if sectionId === "character" && activeProfile}
        <p class="context-note">Currently editing character-scoped settings for <strong>{activeProfile.Name}</strong>.</p>
      {:else if sectionId === "application"}
        <p class="context-note">Account selection remains available after disconnecting from the current game session.</p>
      {:else if sectionId === "themes"}
        <p class="context-note">The full Theme Manager is planned for 0.7.0. CRT effects live here now so appearance settings have a stable home.</p>
      {/if}
    </main>
  </div>
</Modal>

<style>
  .settings-shell {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    height: 100%;
    margin: -12px;
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    border-right: 1px solid var(--border);
    background: var(--bg-elevated);
  }
  nav button {
    text-align: left;
    background: transparent;
    border-color: transparent;
  }
  nav button.active {
    color: var(--accent);
    border-color: var(--accent);
    background: var(--bg-panel);
  }
  nav .menu-home {
    margin-bottom: 8px;
    color: var(--fg-dim);
  }
  .nav-spacer { flex: 1; }
  main {
    min-width: 0;
    padding: 16px;
    overflow: auto;
  }
  .page-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 14px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border);
  }
  h2,
  h3,
  p { margin: 0; }
  h2 {
    color: var(--fg-bright);
    font-size: 18px;
  }
  h3 {
    margin-bottom: 9px;
    color: var(--accent);
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: .8px;
  }
  p,
  small {
    color: var(--fg-dim);
    font-size: 11px;
    line-height: 1.45;
  }
  .scope-label {
    padding: 4px 7px;
    border: 1px solid var(--border);
    color: var(--fg-dim);
    font-size: 10px;
    font-style: normal;
    text-transform: uppercase;
    white-space: nowrap;
  }
  .block {
    margin-bottom: 10px;
    padding: 11px;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
  }
  .scope-copy { margin: -3px 0 9px; }
  .scope-copy strong { color: var(--accent); }
  .profile-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 7px;
  }
  .profile-grid button,
  .entry-list button {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
    padding: 10px 12px;
    text-align: left;
  }
  .profile-grid button > span,
  .entry-list button > span:first-child { flex: 1; min-width: 0; }
  .profile-grid strong,
  .entry-list strong { display: block; color: var(--fg-bright); }
  .profile-grid small,
  .entry-list small { display: block; margin-top: 2px; font-weight: 400; }
  .profile-grid button.chosen { border-color: var(--accent); }
  .profile-grid b {
    color: var(--accent);
    font-size: 10px;
    text-transform: uppercase;
  }
  .entry-list {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 7px;
  }
  .entry-end {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
  }
  .entry-end i {
    color: var(--accent);
    font-size: 20px;
    font-style: normal;
  }
  button:hover small { color: inherit; }
  .context-note { margin-top: 14px; }
  .context-note strong { color: var(--accent); }
  @media (max-width: 760px) {
    .settings-shell { grid-template-columns: 1fr; height: auto; min-height: 0; }
    nav { display: none; }
    .page-head { flex-direction: column; }
    .profile-grid,
    .entry-list { grid-template-columns: 1fr; }
  }
</style>
