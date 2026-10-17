<script lang="ts">
  import Modal from "../Modal.svelte";
  import ExpansionPreview from "../ExpansionPreview.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { CustomTabConfig, DisplayComponentConfig, DisplayProfileConfig, DisplaySidebarConfig } from "../../lib/types";
  import {
    BUILTIN_VIEWS,
    COMPONENT_IDS,
    componentName,
    moveComponent,
    newCustomViewID,
    newSidebar,
  } from "../../lib/layout";

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
      Sidebars: value.Sidebars.map(cloneSidebar),
    };
  }

  let layout = $state(cloneProfile(store.displayProfile()!));
  let custom = $state<CustomTabConfig[]>((store.characterProfile()?.CustomViews ?? []).map(cloneView));
  let selected = $state(0);
  let newName = $state("");
  let draggedID = $state("");
  let saving = $state(false);
  let saveChain: Promise<void> = Promise.resolve();

  const current = $derived(custom[selected]);
  const placed = $derived(new Set([
    ...layout.Main.map((component) => component.ComponentID),
    ...layout.Sidebars.flatMap((sidebar) => sidebar.Components.map((component) => component.ComponentID)),
  ]));
  const available = $derived([
    ...BUILTIN_VIEWS.map((view) => ({ id: view.id, name: view.name })),
    ...custom.map((view) => ({ id: view.ID, name: view.Name })),
  ].filter((component) => !placed.has(component.id)));

  function applyLocal() {
    const registry = store.config!.DisplayProfiles;
    const index = registry.Profiles.findIndex((profile) => profile.ID === registry.DefaultID);
    if (index >= 0) registry.Profiles[index] = layout;
    store.characterProfile()!.CustomViews = custom;
    store.rebuildTabs(custom);
    store.displayMode = layout.SidebarsVisible ? "sidebar" : "off";
  }

  function persist() {
    applyLocal();
    saving = true;
    saveChain = saveChain
      .then(() => api.setViewLayout(layout, custom))
      .catch((error) => store.addToast("Layout save failed", String(error)))
      .finally(() => (saving = false));
  }

  function addView() {
    const name = newName.trim();
    if (!name) return;
    const id = newCustomViewID();
    custom = [...custom, { ID: id, Name: name, Visible: true, EchoCommands: false, Rules: [] }];
    moveComponent(layout, id, { kind: "main" }, layout.Main.length);
    selected = custom.length - 1;
    newName = "";
    persist();
  }

  function removeEverywhere(id: string) {
    layout.Main = layout.Main.filter((component) => component.ComponentID !== id);
    for (const sidebar of layout.Sidebars) {
      sidebar.Components = sidebar.Components.filter((component) => component.ComponentID !== id);
    }
  }

  function removeView(index: number) {
    const id = custom[index]?.ID;
    custom = custom.filter((_, i) => i !== index);
    if (id) removeEverywhere(id);
    if (selected >= custom.length) selected = Math.max(0, custom.length - 1);
    persist();
  }

  function addRule() {
    if (!current) return;
    current.Rules = [...current.Rules, { Pattern: "", Include: true, Active: true }];
    persist();
  }

  function removeRule(index: number) {
    if (!current) return;
    current.Rules = current.Rules.filter((_, i) => i !== index);
    persist();
  }

  function drop(destination: { kind: "main" } | { kind: "sidebar"; sidebarID: string }, index: number, event: DragEvent) {
    event.preventDefault();
    event.stopPropagation();
    const id = draggedID || event.dataTransfer?.getData("text/plain") || "";
    if (!id || id === COMPONENT_IDS.all && destination.kind !== "main") return;
    moveComponent(layout, id, destination, index);
    draggedID = "";
    persist();
  }

  function addByID(destination: { kind: "main" } | { kind: "sidebar"; sidebarID: string }, select: HTMLSelectElement) {
    if (!select.value) return;
    const targetLength = destination.kind === "main"
      ? layout.Main.length
      : layout.Sidebars.find((sidebar) => sidebar.ID === destination.sidebarID)?.Components.length ?? 0;
    moveComponent(layout, select.value, destination, targetLength);
    select.value = "";
    persist();
  }

  function removePlacement(id: string) {
    if (id === COMPONENT_IDS.all) return;
    removeEverywhere(id);
    persist();
  }

  function addSidebar() {
    layout.Sidebars = [...layout.Sidebars, newSidebar(layout.Sidebars.length)];
    persist();
  }

  function removeSidebar(index: number) {
    const sidebar = layout.Sidebars[index];
    if (!sidebar) return;
    for (const component of [...sidebar.Components]) {
      moveComponent(layout, component.ComponentID, { kind: "main" }, layout.Main.length);
    }
    layout.Sidebars = layout.Sidebars.filter((_, i) => i !== index);
    persist();
  }

  function moveSidebar(index: number, delta: -1 | 1) {
    const next = index + delta;
    if (next < 0 || next >= layout.Sidebars.length) return;
    [layout.Sidebars[index], layout.Sidebars[next]] = [layout.Sidebars[next], layout.Sidebars[index]];
    layout.Sidebars = [...layout.Sidebars];
    persist();
  }
</script>

<Modal title="Layout" extraWide back>
  <div class="layout-editor">
    <div class="status">
      <label><input type="checkbox" bind:checked={layout.SidebarsVisible} onchange={persist} /> Show sidebars globally (Alt+S)</label>
      <span class="dim">{saving ? "Saving…" : "Changes save immediately"}</span>
    </div>

    <section>
      <h3>Main tabs</h3>
      <div class="destination main-destination" role="list" ondragover={(event) => event.preventDefault()} ondrop={(event) => drop({ kind: "main" }, layout.Main.length, event)}>
        {#each layout.Main as component, index (component.ComponentID)}
          <div class="component" role="listitem" draggable={component.ComponentID !== COMPONENT_IDS.all}
            ondragstart={(event) => { draggedID = component.ComponentID; event.dataTransfer?.setData("text/plain", component.ComponentID); }}
            ondragover={(event) => event.preventDefault()} ondrop={(event) => drop({ kind: "main" }, index, event)}>
            <span class="grip">⋮⋮</span><strong>{componentName(component.ComponentID, custom)}</strong>
            <code>{component.ComponentID}</code>
            {#if component.ComponentID !== COMPONENT_IDS.all}<button class="danger" aria-label={`Unplace ${componentName(component.ComponentID, custom)}`} onclick={() => removePlacement(component.ComponentID)}>✕</button>{/if}
          </div>
        {/each}
        <div class="add-component"><select aria-label="Component ID to add to main"><option value="">Add component by ID…</option>{#each available as item}<option value={item.id}>{item.name} — {item.id}</option>{/each}</select><button onclick={(event) => addByID({ kind: "main" }, event.currentTarget.previousElementSibling as HTMLSelectElement)}>Add</button></div>
      </div>
    </section>

    <section>
      <div class="section-head"><h3>Sidebars</h3><button onclick={addSidebar}>+ Add sidebar</button></div>
      <div class="sidebar-grid">
        {#each layout.Sidebars as sidebar, sidebarIndex (sidebar.ID)}
          <fieldset>
            <legend>{sidebar.Name}</legend>
            <div class="sidebar-id"><code>{sidebar.ID}</code><span><button aria-label={`Move ${sidebar.Name} earlier`} onclick={() => moveSidebar(sidebarIndex, -1)} disabled={sidebarIndex === 0}>↑</button><button aria-label={`Move ${sidebar.Name} later`} onclick={() => moveSidebar(sidebarIndex, 1)} disabled={sidebarIndex === layout.Sidebars.length - 1}>↓</button><button class="danger" aria-label={`Delete ${sidebar.Name}`} onclick={() => removeSidebar(sidebarIndex)}>✕</button></span></div>
            <label><span>Name</span><input aria-label={`${sidebar.Name} name`} bind:value={sidebar.Name} onchange={persist} /></label>
            <label><span>Enabled</span><input aria-label={`${sidebar.Name} enabled`} type="checkbox" bind:checked={sidebar.Enabled} onchange={persist} /></label>
            <label><span>Side</span><select aria-label={`${sidebar.Name} side`} bind:value={sidebar.Side} onchange={persist}><option value="left">Left</option><option value="right">Right</option></select></label>
            <label><span>Width</span><input aria-label={`${sidebar.Name} width`} type="number" min="180" max="600" bind:value={sidebar.Width} onchange={persist} /></label>
            <div class="destination" role="list" ondragover={(event) => event.preventDefault()} ondrop={(event) => drop({ kind: "sidebar", sidebarID: sidebar.ID }, sidebar.Components.length, event)}>
              {#each sidebar.Components as component, index (component.ComponentID)}
                <div class="component" role="listitem" draggable="true"
                  ondragstart={(event) => { draggedID = component.ComponentID; event.dataTransfer?.setData("text/plain", component.ComponentID); }}
                  ondragover={(event) => event.preventDefault()} ondrop={(event) => drop({ kind: "sidebar", sidebarID: sidebar.ID }, index, event)}>
                  <span class="grip">⋮⋮</span><strong>{componentName(component.ComponentID, custom)}</strong>
                  <input aria-label={`${componentName(component.ComponentID, custom)} height`} type="number" min="80" max="800" bind:value={component.SidebarHeight} onchange={persist} />
                  <button class="danger" aria-label={`Unplace ${componentName(component.ComponentID, custom)}`} onclick={() => removePlacement(component.ComponentID)}>✕</button>
                </div>
              {/each}
              <div class="add-component"><select aria-label={`Component ID to add to ${sidebar.Name}`}><option value="">Add component by ID…</option>{#each available as item}<option value={item.id}>{item.name} — {item.id}</option>{/each}</select><button onclick={(event) => addByID({ kind: "sidebar", sidebarID: sidebar.ID }, event.currentTarget.previousElementSibling as HTMLSelectElement)}>Add</button></div>
            </div>
          </fieldset>
        {/each}
      </div>
    </section>

    <section>
      <h3>Filtered text components</h3>
      <div class="custom-cols">
        <div class="view-list">
          {#each custom as view, index (view.ID)}
            <div class="list-row" class:active={selected === index}>
              <button class="name" onclick={() => (selected = index)}>{view.Name}</button>
              <button class="danger" aria-label={`Delete ${view.Name}`} onclick={() => removeView(index)}>✕</button>
            </div>
          {/each}
          <div class="add-row"><input bind:value={newName} placeholder="New filtered text component…" onkeydown={(event) => { if (event.key === "Enter") { event.preventDefault(); addView(); } }} /><button onclick={addView}>+</button></div>
        </div>
        <div class="custom-editor">
          {#if current}
            <code>{current.ID}</code>
            <label><span>Name</span><input bind:value={current.Name} onchange={persist} /></label>
            <label><span>Enabled</span><input type="checkbox" bind:checked={current.Visible} onchange={persist} /></label>
            <label><span>Route command echoes</span><input type="checkbox" bind:checked={current.EchoCommands} onchange={persist} /></label>
            <div class="rules-head">Rules</div>
            {#each current.Rules as rule, index (index)}
              <div class="rule">
                <button onclick={() => { rule.Include = !rule.Include; persist(); }}>{rule.Include ? "matches" : "excludes"}</button>
                <span><input bind:value={rule.Pattern} placeholder="pattern" onchange={persist} /><ExpansionPreview value={rule.Pattern} match /></span>
                <input type="checkbox" bind:checked={rule.Active} aria-label="Rule active" onchange={persist} />
                <button class="danger" onclick={() => removeRule(index)}>✕</button>
              </div>
            {/each}
            <button onclick={addRule}>+ Add rule</button>
          {:else}<div class="dim empty">Add a filtered text component to begin.</div>{/if}
        </div>
      </div>
    </section>
  </div>
</Modal>

<style>
  .layout-editor { display: flex; flex-direction: column; gap: 18px; }
  .status, .section-head, .sidebar-id { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
  section { display: flex; flex-direction: column; gap: 8px; }
  h3 { margin: 0; color: var(--accent); font-size: 13px; text-transform: uppercase; letter-spacing: 1px; }
  label { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 13px; }
  .sidebar-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 10px; }
  fieldset { min-width: 0; border: 1px solid var(--border); display: flex; flex-direction: column; gap: 7px; }
  legend { color: var(--fg-bright); }
  code { color: var(--fg-dim); font-size: 9px; overflow-wrap: anywhere; }
  input[type="number"] { width: 72px; }
  .destination { min-height: 52px; display: flex; flex-direction: column; gap: 5px; padding: 6px; border: 1px dashed var(--border); }
  .main-destination { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
  .component { min-width: 0; display: grid; grid-template-columns: auto minmax(80px, 1fr) 72px auto; gap: 6px; align-items: center; padding: 6px; border: 1px solid var(--border); background: var(--bg-elevated); }
  .main-destination .component { grid-template-columns: auto minmax(70px, 1fr) minmax(100px, 1.4fr) auto; }
  .grip { cursor: grab; color: var(--accent); }
  .add-component { display: flex; gap: 5px; }
  .add-component select { min-width: 0; flex: 1; }
  .custom-cols { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 14px; min-height: 280px; }
  .view-list { border-right: 1px solid var(--border); padding-right: 10px; display: flex; flex-direction: column; gap: 5px; }
  .list-row, .add-row, .rule { display: flex; align-items: flex-start; gap: 5px; }
  .list-row .name { flex: 1; text-align: left; }
  .list-row.active .name { border-color: var(--accent); color: var(--accent); }
  .add-row { margin-top: 4px; }
  .add-row input { min-width: 0; flex: 1; }
  .custom-editor { display: flex; flex-direction: column; gap: 8px; }
  .custom-editor > label > input:not([type="checkbox"]) { width: 60%; }
  .rules-head { margin-top: 5px; color: var(--fg-dim); font-size: 11px; text-transform: uppercase; }
  .rule > span { flex: 1; min-width: 0; }
  .danger { flex: 0 0 auto; }
  .empty { padding: 30px; text-align: center; }
  @media (max-width: 760px) { .custom-cols { grid-template-columns: 1fr; } .view-list { border-right: 0; border-bottom: 1px solid var(--border); padding: 0 0 10px; } }
</style>
