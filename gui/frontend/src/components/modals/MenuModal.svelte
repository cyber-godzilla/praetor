<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import { MENU_SECTIONS, menuScopeLabel, searchMenuEntries, type MenuEntry } from "../../lib/menu";
  import * as api from "../../lib/bridge";

  const activeProfile = $derived(store.characterProfile()?.Name ?? "Default");
  const activeMode = $derived(store.mode || "disabled");
  let query = $state("");
  const searchResults = $derived(searchMenuEntries(query));

  function open(route: string) {
    store.openModal = route;
  }

  async function playScript() {
    if (store.connState !== "connected") {
      store.addToast("Play", "Not connected — nothing was played.");
      return;
    }
    try {
      const preview = await api.pickPlayFile();
      if (!preview.path) return;
      store.playPreview = preview;
      open("playscript");
    } catch (error) {
      store.addToast("Play", String(error));
    }
  }

  async function sendFile() {
    if (store.connState !== "connected") {
      store.addToast("Send", "Not connected — nothing was sent.");
      return;
    }
    try {
      const preview = await api.pickSendFile();
      if (!preview.path) return;
      store.sendPreview = preview;
      open("sendfile");
    } catch (error) {
      store.addToast("Send", String(error));
    }
  }

  async function reloadScripts() {
    try {
      await api.reloadScripts();
      [store.modeNames, store.modeSpecs] = await Promise.all([api.modeNames(), api.modeSpecs()]);
      store.addToast("Scripts reloaded", "");
    } catch (error) {
      store.addToast("Reload failed", String(error));
    }
  }

  async function activate(entry: MenuEntry) {
    if (entry.route) open(entry.route);
    else if (entry.action === "reload-scripts") await reloadScripts();
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

  async function signOut() {
    store.openModal = null;
    try {
      await api.quit();
    } catch (error) {
      store.addToast("Sign out failed", error instanceof Error ? error.message : String(error));
    }
  }
</script>

<Modal title="Praetor" extraWide>
  <div class="home">
    <section>
      <div class="section-title">Quick actions</div>
      <div class="quick-grid">
        <button onclick={() => open("menu-character")}>
          <span><strong>Character</strong><small>Switch or manage profiles</small></span>
          <b>{activeProfile}</b><i>›</i>
        </button>
        <button onclick={() => open("modeselect")}>
          <span><strong>Mode</strong><small>Change the active automation</small></span>
          <b>{activeMode}</b><i>›</i>
        </button>
        <button onclick={playScript}>
          <span><strong>Play a script…</strong><small>Choose and preview a performance</small></span>
        </button>
        <button onclick={sendFile}>
          <span><strong>Send a file…</strong><small>Expand variables and send text</small></span>
        </button>
        <button aria-label="Reload scripts" onclick={() => void reloadScripts()}>
          <span><strong>Reload scripts</strong><small>Refresh Lua modes and command hints from disk</small></span>
          <i>↻</i>
        </button>
      </div>
    </section>

    <section>
      <label class="menu-search">
        <span class="section-title">Find a setting</span>
        <input type="search" bind:value={query} placeholder="Search settings, tools, and commands…" aria-label="Search Praetor settings" />
      </label>
    </section>

    {#if query.trim()}
      <section>
        <div class="section-title">Search results</div>
        <div class="search-results" aria-live="polite">
          {#each searchResults as result (`${result.section.id}:${result.entry.label}`)}
            <button onclick={() => activate(result.entry)}>
              <span>
                <strong>{result.entry.label}</strong>
                <small>{result.section.label} · {result.entry.description}</small>
              </span>
              <em>{menuScopeLabel(result.entry.scope ?? result.section.scope)}</em>
              <i>{result.entry.route ? "›" : "↻"}</i>
            </button>
          {/each}
          {#if searchResults.length === 0}<div class="no-results">No settings match “{query.trim()}”.</div>{/if}
        </div>
      </section>
    {:else}
    <section>
      <div class="section-title">Manage Praetor</div>
      <div class="home-grid">
        {#each MENU_SECTIONS as section (section.id)}
          <button class="category" onclick={() => open(`menu-${section.id}`)}>
            <span><strong>{section.label}</strong><small>{section.description}</small></span><i>›</i>
          </button>
        {/each}
      </div>
    </section>
    {/if}

    <div class="session-actions">
      {#if api.inWeb()}
        <button onclick={logout}>Disconnect shared game</button>
        <button onclick={signOut}>Sign out of web UI</button>
      {:else}
        <button onclick={logout}>Logout</button>
        <button onclick={() => void api.quit()}>Quit Praetor</button>
      {/if}
    </div>
  </div>
</Modal>

<style>
  .home {
    display: flex;
    flex-direction: column;
    gap: 22px;
    height: 100%;
  }
  section { min-width: 0; }
  .section-title {
    margin-bottom: 8px;
    color: var(--accent);
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .quick-grid,
  .home-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .menu-search { display: block; }
  .menu-search input { width: 100%; box-sizing: border-box; }
  .search-results { display: flex; flex-direction: column; gap: 7px; }
  .search-results button {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 54px;
    padding: 9px 12px;
    text-align: left;
  }
  .search-results button > span { flex: 1; min-width: 0; }
  .search-results em {
    padding: 3px 6px;
    border: 1px solid var(--border);
    color: var(--fg-dim);
    font-size: 9px;
    font-style: normal;
    text-transform: uppercase;
    white-space: nowrap;
  }
  .no-results { padding: 18px; border: 1px dashed var(--border); color: var(--fg-dim); text-align: center; }
  .quick-grid button,
  .category {
    min-height: 58px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    text-align: left;
  }
  .quick-grid button span,
  .category span { flex: 1; min-width: 0; }
  .quick-grid strong,
  .category strong {
    display: block;
    color: var(--fg-bright);
  }
  .quick-grid b {
    max-width: 130px;
    overflow: hidden;
    color: var(--accent);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  button small {
    display: block;
    margin-top: 2px;
    color: var(--fg-dim);
    font-size: 11px;
    font-weight: 400;
  }
  button:hover small { color: inherit; }
  button i {
    color: var(--accent);
    font-size: 20px;
    font-style: normal;
  }
  .session-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: auto;
    padding-top: 14px;
    border-top: 1px solid var(--border);
  }
  @media (max-width: 599px) {
    .home { height: auto; min-height: 0; }
    .quick-grid,
    .home-grid { grid-template-columns: 1fr; }
    .session-actions { flex-wrap: wrap; }
    .session-actions button { min-height: 44px; }
  }
</style>
