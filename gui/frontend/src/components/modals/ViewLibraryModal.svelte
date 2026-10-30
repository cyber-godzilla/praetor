<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import ExpansionPreview from "../ExpansionPreview.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { CustomTabConfig, DisplayComponentConfig, DisplayProfileConfig, DisplaySidebarConfig } from "../../lib/types";
  import { moveComponent, newCustomViewID } from "../../lib/layout";

  function cloneView(view: CustomTabConfig): CustomTabConfig {
    return { ...view, Rules: (view.Rules ?? []).map((rule) => ({ ...rule })) };
  }
  function cloneComponent(component: DisplayComponentConfig): DisplayComponentConfig {
    return { ...component };
  }
  function cloneSidebar(sidebar: DisplaySidebarConfig): DisplaySidebarConfig {
    return { ...sidebar, Components: sidebar.Components.map(cloneComponent) };
  }
  function cloneProfile(value: DisplayProfileConfig): DisplayProfileConfig {
    return {
      ...value,
      Main: value.Main.map(cloneComponent),
      Bench: (value.Bench ?? []).map(cloneComponent),
      Sidebars: value.Sidebars.map(cloneSidebar),
    };
  }

  const profiles = $derived(store.config?.CharacterProfiles?.Profiles ?? []);
  let ownerID = $state(untrack(() => store.activeCharacterProfileID || store.config?.CharacterProfiles?.DefaultID || profiles[0]?.ID || ""));

  function profileFor(id: string) {
    return profiles.find((profile) => profile.ID === id);
  }
  function layoutFor(id: string) {
    const registry = store.config?.DisplayProfiles;
    const defaultCharacterID = store.config?.CharacterProfiles?.DefaultID;
    if (!registry) return undefined;
    if (id !== defaultCharacterID) {
      const characterLayout = registry.Profiles.find((profile) => profile.CharacterProfileID === id);
      if (characterLayout) return characterLayout;
    }
    return registry.Profiles.find((profile) => profile.ID === registry.DefaultID) ?? registry.Profiles[0];
  }

  let layout = $state(untrack(() => cloneProfile(layoutFor(ownerID)!)));
  let views = $state<CustomTabConfig[]>(untrack(() => (profileFor(ownerID)?.CustomViews ?? []).map(cloneView)));
  let selected = $state(0);
  let newName = $state("");
  let saving = $state(false);
  let saveChain: Promise<void> = Promise.resolve();

  const current = $derived(views[selected]);
  const owner = $derived(profileFor(ownerID));
  const allViews = $derived(profiles.flatMap((profile) =>
    (profile.ID === ownerID ? views : (profile.CustomViews ?? [])).map((view) => ({ profile, view })),
  ));

  function loadOwner(id: string, viewID?: string) {
    const nextLayout = layoutFor(id);
    if (!nextLayout) return;
    ownerID = id;
    layout = cloneProfile(nextLayout);
    views = (profileFor(id)?.CustomViews ?? []).map(cloneView);
    selected = viewID ? Math.max(0, views.findIndex((view) => view.ID === viewID)) : 0;
    newName = "";
  }

  function usage(profileID: string, viewID: string): string {
    const profile = profileFor(profileID);
    const target = profileID === ownerID ? layout : layoutFor(profileID);
    if (!target) return profile?.Name ?? "Unknown character";
    const placements: string[] = [];
    if (target.Main.some((component) => component.ComponentID === viewID)) placements.push("main tabs");
    for (const sidebar of target.Sidebars) {
      if (sidebar.Components.some((component) => component.ComponentID === viewID)) placements.push(sidebar.Name);
    }
    if (target.Bench.some((component) => component.ComponentID === viewID)) placements.push("Bench");
    return `${profile?.Name ?? "Unknown character"} · ${placements.join(", ") || "unplaced"}`;
  }

  function applyLocal() {
    const registry = store.config!.DisplayProfiles;
    const index = registry.Profiles.findIndex((profile) => profile.ID === layout.ID);
    if (index >= 0) registry.Profiles[index] = layout;
    const profile = profileFor(ownerID);
    if (profile) profile.CustomViews = views;
    if (ownerID === store.activeCharacterProfileID) store.rebuildTabs(views);
  }

  function persist() {
    applyLocal();
    const profileID = ownerID;
    const nextLayout = cloneProfile(layout);
    const nextViews = views.map(cloneView);
    saving = true;
    saveChain = saveChain
      .then(() => api.setProfileViewLayout(profileID, nextLayout, nextViews))
      .catch((error) => store.addToast("View save failed", String(error)))
      .finally(() => (saving = false));
  }

  function addView() {
    const name = newName.trim();
    if (!name) return;
    const id = newCustomViewID();
    views = [...views, { ID: id, Name: name, Visible: true, EchoCommands: false, Rules: [] }];
    moveComponent(layout, id, { kind: "bench" }, layout.Bench.length);
    selected = views.length - 1;
    newName = "";
    persist();
  }

  function removeEverywhere(id: string) {
    layout.Main = layout.Main.filter((component) => component.ComponentID !== id);
    layout.Bench = layout.Bench.filter((component) => component.ComponentID !== id);
    for (const sidebar of layout.Sidebars) {
      sidebar.Components = sidebar.Components.filter((component) => component.ComponentID !== id);
    }
  }

  function removeView(index: number) {
    const id = views[index]?.ID;
    views = views.filter((_, viewIndex) => viewIndex !== index);
    if (id) removeEverywhere(id);
    if (selected >= views.length) selected = Math.max(0, views.length - 1);
    persist();
  }

  function addRule() {
    if (!current) return;
    current.Rules = [...current.Rules, { Pattern: "", Include: true, Active: true }];
    persist();
  }

  function removeRule(index: number) {
    if (!current) return;
    current.Rules = current.Rules.filter((_, ruleIndex) => ruleIndex !== index);
    persist();
  }
</script>

<Modal title="View Library" wide back>
  <div class="library">
    <div class="section-head">
      <div>
        <h3>Filtered text views</h3>
        <div class="dim hint">This global library shows every character's views and where each is used. New views start on that character's Layout Bench.</div>
      </div>
      <span class="dim">{saving ? "Saving…" : "Changes save immediately"}</span>
    </div>

    <div class="view-cols">
      <div class="view-list">
        {#each allViews as item (item.view.ID)}
          <div class="list-row" class:active={ownerID === item.profile.ID && current?.ID === item.view.ID}>
            <button class="name" onclick={() => loadOwner(item.profile.ID, item.view.ID)}>
              <strong>{item.view.Name}</strong>
              <small>{usage(item.profile.ID, item.view.ID)}</small>
            </button>
            {#if ownerID === item.profile.ID && current?.ID === item.view.ID}
              <button class="danger" aria-label={`Delete ${item.view.Name}`} onclick={() => removeView(selected)}>✕</button>
            {/if}
          </div>
        {/each}
        {#if allViews.length === 0}<div class="dim empty-list">No filtered text views yet.</div>{/if}
        <div class="add-row">
          <input bind:value={newName} placeholder="New filtered text view…" onkeydown={(event) => { if (event.key === "Enter") { event.preventDefault(); addView(); } }} />
          <button aria-label="Add filtered text view" onclick={addView}>+</button>
        </div>
      </div>

      <div class="view-editor">
        <label class="owner"><span>Character</span><select value={ownerID} onchange={(event) => loadOwner(event.currentTarget.value)}>{#each profiles as profile (profile.ID)}<option value={profile.ID}>{profile.Name}</option>{/each}</select></label>
        {#if current}
          <label><span>Name</span><input bind:value={current.Name} onchange={persist} /></label>
          <label><span>Enabled</span><input type="checkbox" bind:checked={current.Visible} onchange={persist} /></label>
          <label><span>Route command echoes</span><input type="checkbox" bind:checked={current.EchoCommands} onchange={persist} /></label>
          <div class="rules-head">Rules</div>
          {#each current.Rules as rule, index (index)}
            <div class="rule">
              <button onclick={() => { rule.Include = !rule.Include; persist(); }}>{rule.Include ? "matches" : "excludes"}</button>
              <span><input bind:value={rule.Pattern} placeholder="pattern" onchange={persist} /><ExpansionPreview value={rule.Pattern} match /></span>
              <input type="checkbox" bind:checked={rule.Active} aria-label="Rule active" onchange={persist} />
              <button class="danger" aria-label={`Delete rule ${index + 1}`} onclick={() => removeRule(index)}>✕</button>
            </div>
          {/each}
          <button onclick={addRule}>+ Add rule</button>
        {:else}
          <div class="dim empty">Add a filtered text view for {owner?.Name ?? "this character"} to begin.</div>
        {/if}
      </div>
    </div>
  </div>
</Modal>

<style>
  .library { display: flex; flex-direction: column; gap: 14px; }
  .section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
  h3 { margin: 0; color: var(--accent); font-size: 13px; text-transform: uppercase; letter-spacing: 1px; }
  .hint { margin-top: 4px; max-width: 520px; font-size: 11px; line-height: 1.4; }
  .view-cols { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 14px; min-height: 280px; }
  .view-list { border-right: 1px solid var(--border); padding-right: 10px; display: flex; flex-direction: column; gap: 5px; }
  .list-row, .add-row, .rule { display: flex; align-items: flex-start; gap: 5px; }
  .list-row .name { flex: 1; min-width: 0; text-align: left; }
  .list-row .name strong, .list-row .name small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .list-row .name small { margin-top: 2px; color: var(--fg-dim); font-size: 9px; }
  .list-row.active .name { border-color: var(--accent); color: var(--accent); }
  .add-row { margin-top: 4px; }
  .add-row input { min-width: 0; flex: 1; }
  .view-editor { display: flex; flex-direction: column; gap: 8px; }
  .owner { padding-bottom: 8px; border-bottom: 1px solid var(--border); }
  .view-editor > label { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 13px; }
  .view-editor > label > input:not([type="checkbox"]) { width: 60%; }
  .rules-head { margin-top: 5px; color: var(--fg-dim); font-size: 11px; text-transform: uppercase; }
  .rule > span { flex: 1; min-width: 0; }
  .danger { flex: 0 0 auto; }
  .empty { padding: 30px; text-align: center; }
  .empty-list { padding: 14px 5px; font-size: 11px; text-align: center; }
  @media (max-width: 760px) {
    .section-head { flex-direction: column; }
    .view-cols { grid-template-columns: 1fr; }
    .view-list { border-right: 0; border-bottom: 1px solid var(--border); padding: 0 0 10px; }
  }
</style>
