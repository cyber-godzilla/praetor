<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { LocalScriptDirectoryView, ScriptManagerState } from "../../lib/types";

  let managerState: ScriptManagerState = $state({
    localDirectories: [], managed: [], suggestedURL: "", suggestedInstalled: false,
  });
  let localDirectories: LocalScriptDirectoryView[] = $state([]);
  let localDraft = $state("");
  let repositoryURL = $state("");
  let busy = $state("");
  let loaded = $state(false);

  $effect(() => {
    if (!loaded) {
      loaded = true;
      void refresh(true);
    }
  });

  async function refresh(resetLocal = false) {
    try {
      managerState = await api.getScriptManager();
      if (resetLocal) localDirectories = managerState.localDirectories.map((directory) => ({ ...directory }));
    } catch (error) {
      toastError("Unable to load Script Manager", error);
    }
  }

  async function install(url: string) {
    const trimmed = url.trim();
    if (!trimmed || busy) return;
    busy = "install";
    try {
      await api.installManagedScript(trimmed);
      repositoryURL = "";
      await refresh();
      store.addToast("Script source installed", "Installed disabled and restricted.");
    } catch (error) {
      toastError("Install failed", error);
    } finally {
      busy = "";
    }
  }

  async function check(id: string) {
    busy = id + ":check";
    try {
      const status = await api.checkManagedScript(id);
      await refresh();
      store.addToast(
        status.updated ? "Script source updated" : "Update check complete",
        status.updated ? `${status.version || status.ref} is installed and ready.` : "Up to date.",
      );
    } catch (error) {
      await refresh();
      toastError("Update check failed", error);
    } finally {
      busy = "";
    }
  }

  async function remove(id: string, name: string) {
    if (!window.confirm(`Remove ${name} and its managed checkout?`)) return;
    busy = id + ":remove";
    try {
      await api.removeManagedScript(id);
      await refresh();
    } catch (error) {
      toastError("Remove failed", error);
    } finally {
      busy = "";
    }
  }

  async function setStartup(id: string, enabled: boolean) {
    busy = id + ":startup";
    try {
      await api.setManagedScriptStartupCheck(id, enabled);
      await refresh();
    } catch (error) {
      toastError("Setting failed", error);
      await refresh();
    } finally {
      busy = "";
    }
  }

  async function setEnabled(id: string, name: string, enabled: boolean, stopAutomation = false) {
    if (enabled && !stopAutomation && !window.confirm(
      `Enable ${name}? Restricted scripts cannot access files or processes, but they can send game commands through Praetor's automation APIs.`,
    )) return;
    busy = id + ":enabled";
    try {
      await api.setManagedScriptEnabled(id, enabled, stopAutomation);
      await refresh();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (!stopAutomation && message.includes("stop automation") && window.confirm(`${message}\n\nStop the active Lua mode and continue?`)) {
        busy = "";
        await setEnabled(id, name, enabled, true);
        return;
      }
      toastError(enabled ? "Enable failed" : "Disable failed", error);
    } finally {
      busy = "";
    }
  }

  async function setTrust(id: string, name: string, trust: "restricted" | "trusted", stopAutomation = false) {
    if (trust === "trusted" && !stopAutomation && !window.confirm(
      `Trust ${name}? Trusted scripts can read and modify files, inspect the process environment, and launch processes with Praetor's permissions.`,
    )) return;
    busy = id + ":trust";
    try {
      await api.setManagedScriptTrust(id, trust, stopAutomation);
      await refresh();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (!stopAutomation && message.includes("stop automation") && window.confirm(`${message}\n\nStop the active Lua mode and continue?`)) {
        busy = "";
        await setTrust(id, name, trust, true);
        return;
      }
      toastError("Trust change failed", error);
    } finally {
      busy = "";
    }
  }

  async function move(index: number, delta: number) {
    const next = index + delta;
    if (next < 0 || next >= managerState.managed.length || busy) return;
    const reordered = [...managerState.managed];
    [reordered[index], reordered[next]] = [reordered[next], reordered[index]];
    busy = "order";
    try {
      await api.reorderManagedScripts(reordered.map((source) => source.id));
      managerState = { ...managerState, managed: reordered };
    } catch (error) {
      toastError("Reorder failed", error);
    } finally {
      busy = "";
    }
  }

  function addLocal(path = localDraft) {
    const trimmed = path.trim();
    if (trimmed && !localDirectories.some((directory) => directory.path === trimmed)) {
      localDirectories = [...localDirectories, { path: trimmed, enabled: true }];
    }
    localDraft = "";
  }

  function moveLocal(index: number, delta: number) {
    const next = index + delta;
    if (next < 0 || next >= localDirectories.length) return;
    const reordered = [...localDirectories];
    const current = reordered[index];
    const replacement = reordered[next];
    if (current === undefined || replacement === undefined) return;
    reordered[index] = replacement;
    reordered[next] = current;
    localDirectories = reordered;
  }

  async function browseLocal() {
    const path = await api.pickScriptDir();
    if (path) addLocal(path);
  }

  async function saveLocal() {
    busy = "local";
    try {
      const paths = localDirectories.map((directory) => directory.path);
      await api.setScriptDirs(paths);
      if (store.config) store.config.Scripts = [...paths];
      managerState = { ...managerState, localDirectories: localDirectories.map((directory) => ({ ...directory })) };
      store.addToast("Local script directories", "Saved and reloaded.");
    } catch (error) {
      toastError("Save failed", error);
    } finally {
      busy = "";
    }
  }

  async function setLocalEnabled(directory: LocalScriptDirectoryView, enabled: boolean, stopAutomation = false) {
    busy = directory.path + ":enabled";
    try {
      await api.setLocalScriptEnabled(directory.path, enabled, stopAutomation);
      localDirectories = localDirectories.map((candidate) =>
        candidate.path === directory.path ? { ...candidate, enabled } : candidate,
      );
      managerState = {
        ...managerState,
        localDirectories: managerState.localDirectories.map((candidate) =>
          candidate.path === directory.path ? { ...candidate, enabled } : candidate,
        ),
      };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (!stopAutomation && message.includes("stop automation") && window.confirm(`${message}\n\nStop the active Lua mode and continue?`)) {
        busy = "";
        await setLocalEnabled(directory, enabled, true);
        return;
      }
      toastError(enabled ? "Enable failed" : "Disable failed", error);
    } finally {
      busy = "";
    }
  }

  function toastError(title: string, error: unknown) {
    store.addToast(title, error instanceof Error ? error.message : String(error));
  }

  function shortCommit(commit: string) {
    return commit ? commit.slice(0, 10) : "unknown";
  }
</script>

<Modal title="Script Manager" wide back>
  <p class="intro">
    Add Git-backed script collections or keep using local directories. Managed sources install disabled and Restricted, with isolated libraries and source-scoped imports.
  </p>

  <section>
    <div class="heading-row">
      <div>
        <h3>Managed sources</h3>
        <p>Check installs available updates automatically and restores the source to its current enabled state.</p>
      </div>
      {#if managerState.suggestedURL && !managerState.suggestedInstalled}
        <button class="suggested" disabled={!!busy} onclick={() => install(managerState.suggestedURL)}>Add praetor-scripts</button>
      {/if}
    </div>

    <div class="source-list">
      {#each managerState.managed as source, index (source.id)}
        <article class="source">
          <div class="source-top">
            <div class="identity">
              <strong>{source.name}</strong>
              <span class="badges"><span>{source.trust}</span><span>{source.enabled ? "enabled" : "disabled"}</span></span>
            </div>
            <div class="order">
              <button aria-label={`Move ${source.name} up`} disabled={index === 0 || !!busy} onclick={() => move(index, -1)}>↑</button>
              <button aria-label={`Move ${source.name} down`} disabled={index === managerState.managed.length - 1 || !!busy} onclick={() => move(index, 1)}>↓</button>
            </div>
          </div>
          <div class="url">{source.url}</div>
          <div class="details">
            <span>{source.tracking}: {source.installedVersion || source.ref}</span>
            <span>commit {shortCommit(source.installedCommit)}</span>
          </div>
          {#if source.status.error}<p class="error">{source.status.error}</p>{/if}
          {#if source.status.available}
            <p class="available">Update available: {source.status.version || source.status.ref}</p>
          {/if}
          <div class="source-actions">
            <label><input type="checkbox" checked={source.checkOnStartup} disabled={!!busy} onchange={(event) => setStartup(source.id, event.currentTarget.checked)} /> Check at startup</label>
            <button disabled={!!busy || !source.canEnable} onclick={() => setEnabled(source.id, source.name, !source.enabled)}>{source.enabled ? "Disable" : "Enable"}</button>
            <button disabled={!!busy} onclick={() => setTrust(source.id, source.name, source.trust === "trusted" ? "restricted" : "trusted")}>{source.trust === "trusted" ? "Make Restricted" : "Make Trusted…"}</button>
            <span class="spacer"></span>
            <button disabled={!!busy} onclick={() => check(source.id)}>Check for updates</button>
            <button class="danger" disabled={!!busy} onclick={() => remove(source.id, source.name)}>Remove</button>
          </div>
        </article>
      {/each}
      {#if managerState.managed.length === 0}<div class="empty">No managed script sources installed.</div>{/if}
    </div>

    <div class="add-row">
      <input type="url" bind:value={repositoryURL} placeholder="https://example.com/owner/scripts.git" onkeydown={(event) => event.key === "Enter" && install(repositoryURL)} />
      <button disabled={!repositoryURL.trim() || !!busy} onclick={() => install(repositoryURL)}>Add repository</button>
    </div>
  </section>

  <section>
    <div class="heading-row">
      <div><h3>Local directories</h3><p>Existing folders remain fully trusted and reload when saved.</p></div>
    </div>
    <div class="local-list">
      {#each localDirectories as directory, index (directory.path)}
        <div class="local-row">
          <code>{directory.path}</code><span class="badges"><span>{directory.enabled ? "enabled" : "disabled"}</span></span><span class="spacer"></span>
          <button
            disabled={!!busy || !managerState.localDirectories.some((candidate) => candidate.path === directory.path)}
            title={managerState.localDirectories.some((candidate) => candidate.path === directory.path) ? undefined : "Save directories before changing enablement"}
            onclick={() => setLocalEnabled(directory, !directory.enabled)}
          >{directory.enabled ? "Disable" : "Enable"}</button>
          <button aria-label={`Move ${directory.path} up`} disabled={index === 0 || !!busy} onclick={() => moveLocal(index, -1)}>↑</button>
          <button aria-label={`Move ${directory.path} down`} disabled={index === localDirectories.length - 1 || !!busy} onclick={() => moveLocal(index, 1)}>↓</button>
          <button class="danger" onclick={() => localDirectories = localDirectories.filter((_, i) => i !== index)}>Remove</button>
        </div>
      {/each}
      {#if localDirectories.length === 0}<div class="empty">Using Praetor's default local scripts directory.</div>{/if}
    </div>
    <div class="add-row">
      <input bind:value={localDraft} placeholder={api.inWeb() ? "Path on the server" : "Local folder"} onkeydown={(event) => event.key === "Enter" && addLocal()} />
      <button disabled={!localDraft.trim()} onclick={() => addLocal()}>Add</button>
      {#if !api.inWeb()}<button onclick={browseLocal}>Browse…</button>{/if}
      <button class="primary" disabled={busy === "local"} onclick={saveLocal}>Save directories</button>
    </div>
  </section>
</Modal>

<style>
  .intro { margin: 0 0 18px; color: var(--fg-dim); font-size: 12px; line-height: 1.5; }
  section + section { margin-top: 22px; padding-top: 18px; border-top: 1px solid var(--border); }
  h3 { margin: 0; color: var(--amber); font-size: 14px; text-transform: uppercase; letter-spacing: .06em; }
  .heading-row { display: flex; align-items: end; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
  .heading-row p { margin: 3px 0 0; color: var(--fg-dim); font-size: 11px; }
  .source-list, .local-list { display: flex; flex-direction: column; gap: 7px; }
  .source { padding: 10px; background: var(--bg-elevated); border: 1px solid var(--border); border-radius: 5px; }
  .source-top, .identity, .details, .source-actions, .local-row, .add-row { display: flex; align-items: center; gap: 8px; }
  .source-top { justify-content: space-between; }
  .identity strong { font-size: 14px; }
  .badges { display: flex; gap: 5px; }
  .badges span { padding: 1px 5px; border: 1px solid var(--border); border-radius: 8px; color: var(--fg-dim); font-size: 9px; text-transform: uppercase; }
  .order { display: flex; gap: 3px; }
  .order button { min-width: 28px; padding: 2px 7px; }
  .url { margin-top: 5px; overflow: hidden; color: var(--fg-dim); font-family: var(--mono); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
  .details { margin-top: 6px; color: var(--fg-dim); font-size: 11px; }
  .source-actions { margin-top: 9px; flex-wrap: wrap; }
  .source-actions label { display: flex; align-items: center; gap: 5px; font-size: 11px; }
  .source-actions button, .local-row button { padding: 3px 8px; font-size: 11px; }
  .available { margin: 7px 0 0; color: var(--green, #7ecb7e); font-size: 11px; }
  .error { margin: 7px 0 0; color: var(--red, #e87878); font-size: 11px; }
  .spacer { flex: 1; }
  .add-row { margin-top: 9px; }
  .add-row input { flex: 1; min-width: 0; }
  .local-row { justify-content: space-between; padding: 7px 9px; background: var(--bg-elevated); border-radius: 5px; }
  .local-row code { overflow: hidden; color: var(--fg); text-overflow: ellipsis; white-space: nowrap; }
  .empty { padding: 14px; color: var(--fg-dim); text-align: center; font-size: 12px; }
</style>
