<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { DisplayComponentConfig, DisplayProfileConfig, DisplaySidebarConfig } from "../../lib/types";
  import {
    type ComponentDestination,
    COMPONENT_IDS,
    benchComponents,
    componentName,
    componentType,
    deleteSidebar,
    layoutComponentVisible,
    moveComponent,
    newSidebar,
  } from "../../lib/layout";

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

  let layout = $state(cloneProfile(store.displayProfile()!));
  let componentQuery = $state("");
  let draggedID = $state("");
  type PlacementEdge = "before" | "after" | "end";
  type PlacementTarget = {
    destination: ComponentDestination;
    index: number;
    anchorID: string;
    edge: PlacementEdge;
  };
  let placementTarget = $state<PlacementTarget | null>(null);
  let saving = $state(false);
  let saveChain: Promise<void> = Promise.resolve();

  const custom = $derived(store.characterProfile()?.CustomViews ?? []);
  const currentCharacterID = $derived(store.activeCharacterProfileID || store.config?.CharacterProfiles.DefaultID);
  const namedCustom = $derived([
    ...(store.config?.CharacterProfiles.Profiles ?? [])
      .filter((profile) => profile.ID !== currentCharacterID)
      .flatMap((profile) => profile.CustomViews ?? []),
    ...custom,
  ]);
  const bench = $derived(
    benchComponents(layout, namedCustom).filter((component) => layoutComponentVisible(component.id, store.debug)),
  );
  const visibleBench = $derived(bench.filter((component) => {
    const query = componentQuery.trim().toLocaleLowerCase();
    return !query || component.name.toLocaleLowerCase().includes(query);
  }));

  function nameFor(id: string): string {
    return componentName(id, namedCustom);
  }

  function typeFor(id: string) {
    return componentType(id, namedCustom);
  }

  function applyLocal() {
    const registry = store.config!.DisplayProfiles;
    const index = registry.Profiles.findIndex((profile) => profile.ID === layout.ID);
    if (index >= 0) registry.Profiles[index] = layout;
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

  function sameDestination(left: ComponentDestination, right: ComponentDestination): boolean {
    if (left.kind !== right.kind) return false;
    return left.kind !== "sidebar" || right.kind === "sidebar" && left.sidebarID === right.sidebarID;
  }

  function placementEdge(destination: ComponentDestination, anchorID: string): PlacementEdge | "" {
    if (!placementTarget || !sameDestination(placementTarget.destination, destination)) return "";
    return placementTarget.anchorID === anchorID ? placementTarget.edge : "";
  }

  function placementAtEnd(destination: ComponentDestination): boolean {
    return !!placementTarget
      && placementTarget.edge === "end"
      && sameDestination(placementTarget.destination, destination);
  }

  function previewPlacement(
    destination: ComponentDestination,
    index: number,
    anchorID: string,
    event: DragEvent,
  ) {
    event.preventDefault();
    event.stopPropagation();
    if (!draggedID) return;
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    const bounds = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const after = destination.kind === "main"
      ? event.clientX >= bounds.left + bounds.width / 2
      : event.clientY >= bounds.top + bounds.height / 2;
    placementTarget = {
      destination,
      index: index + (after ? 1 : 0),
      anchorID,
      edge: after ? "after" : "before",
    };
  }

  function previewEnd(destination: ComponentDestination, index: number, event: DragEvent) {
    event.preventDefault();
    if (!draggedID) return;
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    placementTarget = { destination, index, anchorID: "", edge: "end" };
  }

  function startDrag(id: string, event: DragEvent) {
    draggedID = id;
    placementTarget = null;
    event.dataTransfer?.setData("text/plain", id);
  }

  function finishDrag() {
    draggedID = "";
    placementTarget = null;
  }

  function drop(destination: ComponentDestination, index: number, event: DragEvent) {
    event.preventDefault();
    event.stopPropagation();
    const id = draggedID || event.dataTransfer?.getData("text/plain") || "";
    if (!id || id === COMPONENT_IDS.all && destination.kind !== "main") return;
    const placementIndex = placementTarget && sameDestination(placementTarget.destination, destination)
      ? placementTarget.index
      : index;
    moveComponent(layout, id, destination, placementIndex);
    finishDrag();
    persist();
  }

  function removePlacement(id: string) {
    if (id === COMPONENT_IDS.all) return;
    moveComponent(layout, id, { kind: "bench" }, layout.Bench.length);
    persist();
  }

  function addSidebar() {
    layout.Sidebars = [...layout.Sidebars, newSidebar(layout.Sidebars.length)];
    persist();
  }

  function removeSidebar(index: number) {
    const sidebar = layout.Sidebars[index];
    if (!sidebar) return;
    deleteSidebar(layout, sidebar.ID);
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

    <section class="bench-section">
      <div class="section-head">
        <div><h3>Bench</h3><div class="dim browser-hint">Views not currently displayed live here. Drag one into Main tabs or a sidebar; drop a placed view here to bench it.</div></div>
        <input class="component-search" aria-label="Filter benched views" bind:value={componentQuery} placeholder="Filter Bench…" />
      </div>
      <div class="component-browser destination" class:drop-at-end={placementAtEnd({ kind: "bench" })} role="list" aria-label="View Bench"
        ondragover={(event) => previewEnd({ kind: "bench" }, layout.Bench.length, event)} ondrop={(event) => drop({ kind: "bench" }, layout.Bench.length, event)}>
        {#each visibleBench as item (item.id)}
          <div class="browser-component" class:drop-before={placementEdge({ kind: "bench" }, item.id) === "before"} class:drop-after={placementEdge({ kind: "bench" }, item.id) === "after"} role="listitem" draggable="true"
            ondragstart={(event) => startDrag(item.id, event)} ondragend={finishDrag}
            ondragover={(event) => previewPlacement({ kind: "bench" }, layout.Bench.findIndex((component) => component.ComponentID === item.id), item.id, event)}
            ondrop={(event) => drop({ kind: "bench" }, layout.Bench.findIndex((component) => component.ComponentID === item.id), event)}>
            <span class="grip">⋮⋮</span>
            <div class="component-copy">
              <strong>{item.name}</strong>
              <div class="component-type"><span>Type:</span> {typeFor(item.id)}</div>
            </div>
          </div>
        {/each}
        {#if visibleBench.length === 0}
          <div class="dim empty">{bench.length === 0 ? "All views are placed." : "No benched views match that filter."}</div>
        {/if}
      </div>
    </section>

    <section class="main-section">
      <h3>Main tabs</h3>
      <div class="destination main-destination" class:drop-at-end={placementAtEnd({ kind: "main" })} role="list" ondragover={(event) => previewEnd({ kind: "main" }, layout.Main.length, event)} ondrop={(event) => drop({ kind: "main" }, layout.Main.length, event)}>
        {#each layout.Main as component, index (component.ComponentID)}
          {#if layoutComponentVisible(component.ComponentID, store.debug)}
            <div class="component" class:drop-before={placementEdge({ kind: "main" }, component.ComponentID) === "before"} class:drop-after={placementEdge({ kind: "main" }, component.ComponentID) === "after"} role="listitem" draggable={component.ComponentID !== COMPONENT_IDS.all}
              ondragstart={(event) => startDrag(component.ComponentID, event)} ondragend={finishDrag}
              ondragover={(event) => previewPlacement({ kind: "main" }, index, component.ComponentID, event)} ondrop={(event) => drop({ kind: "main" }, index, event)}>
              <span class="grip">⋮⋮</span>
              <div class="component-copy">
                <strong>{nameFor(component.ComponentID)}</strong>
                <div class="component-type"><span>Type:</span> {typeFor(component.ComponentID)}</div>
              </div>
              {#if component.ComponentID !== COMPONENT_IDS.all}<button class="danger" aria-label={`Unplace ${nameFor(component.ComponentID)}`} onclick={() => removePlacement(component.ComponentID)}>✕</button>{/if}
            </div>
          {/if}
        {/each}
      </div>
    </section>

    <section class="sidebars-section">
      <div class="section-head"><h3>Sidebars</h3><button onclick={addSidebar}>+ Add sidebar</button></div>
      <div class="sidebar-grid">
        {#each layout.Sidebars as sidebar, sidebarIndex (sidebar.ID)}
          <fieldset>
            <legend>{sidebar.Name}</legend>
            <div class="sidebar-actions"><button aria-label={`Move ${sidebar.Name} earlier`} onclick={() => moveSidebar(sidebarIndex, -1)} disabled={sidebarIndex === 0}>↑</button><button aria-label={`Move ${sidebar.Name} later`} onclick={() => moveSidebar(sidebarIndex, 1)} disabled={sidebarIndex === layout.Sidebars.length - 1}>↓</button><button class="danger" aria-label={`Delete ${sidebar.Name}`} onclick={() => removeSidebar(sidebarIndex)}>✕</button></div>
            <label><span>Name</span><input aria-label={`${sidebar.Name} name`} bind:value={sidebar.Name} onchange={persist} /></label>
            <label><span>Enabled</span><input aria-label={`${sidebar.Name} enabled`} type="checkbox" bind:checked={sidebar.Enabled} onchange={persist} /></label>
            <label><span>Side</span><select aria-label={`${sidebar.Name} side`} bind:value={sidebar.Side} onchange={persist}><option value="left">Left</option><option value="right">Right</option></select></label>
            <label><span>Width</span><input aria-label={`${sidebar.Name} width`} type="number" min="180" max="600" bind:value={sidebar.Width} onchange={persist} /></label>
            <div class="destination" class:drop-at-end={placementAtEnd({ kind: "sidebar", sidebarID: sidebar.ID })} role="list" ondragover={(event) => previewEnd({ kind: "sidebar", sidebarID: sidebar.ID }, sidebar.Components.length, event)} ondrop={(event) => drop({ kind: "sidebar", sidebarID: sidebar.ID }, sidebar.Components.length, event)}>
              {#each sidebar.Components as component, index (component.ComponentID)}
                {#if layoutComponentVisible(component.ComponentID, store.debug)}
                  <div class="component" class:drop-before={placementEdge({ kind: "sidebar", sidebarID: sidebar.ID }, component.ComponentID) === "before"} class:drop-after={placementEdge({ kind: "sidebar", sidebarID: sidebar.ID }, component.ComponentID) === "after"} role="listitem" draggable="true"
                    ondragstart={(event) => startDrag(component.ComponentID, event)} ondragend={finishDrag}
                    ondragover={(event) => previewPlacement({ kind: "sidebar", sidebarID: sidebar.ID }, index, component.ComponentID, event)} ondrop={(event) => drop({ kind: "sidebar", sidebarID: sidebar.ID }, index, event)}>
                    <span class="grip">⋮⋮</span>
                    <div class="component-copy">
                      <strong>{nameFor(component.ComponentID)}</strong>
                      <div class="component-type"><span>Type:</span> {typeFor(component.ComponentID)}</div>
                    </div>
                    <input aria-label={`${nameFor(component.ComponentID)} height`} type="number" min="80" max="800" bind:value={component.SidebarHeight} onchange={persist} />
                    <button class="danger" aria-label={`Unplace ${nameFor(component.ComponentID)}`} onclick={() => removePlacement(component.ComponentID)}>✕</button>
                  </div>
                {/if}
              {/each}
            </div>
          </fieldset>
        {/each}
      </div>
    </section>

  </div>
</Modal>

<style>
  .layout-editor { display: grid; grid-template-columns: minmax(0, 1fr) minmax(250px, 320px); gap: 18px; align-items: start; }
  .status { grid-column: 1 / -1; }
  .main-section { grid-column: 1; grid-row: 2; }
  .sidebars-section { grid-column: 1; grid-row: 3; }
  .bench-section { grid-column: 2; grid-row: 2 / span 2; position: sticky; top: 0; max-height: calc(86vh - 110px); min-height: 0; overflow: hidden; }
  .status, .section-head, .sidebar-actions { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
  .sidebar-actions { justify-content: flex-end; }
  section { display: flex; flex-direction: column; gap: 8px; }
  h3 { margin: 0; color: var(--accent); font-size: 13px; text-transform: uppercase; letter-spacing: 1px; }
  label { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 13px; }
  .sidebar-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 10px; }
  .bench-section .section-head { align-items: stretch; flex-direction: column; }
  .browser-hint { margin-top: 3px; font-size: 11px; line-height: 1.35; }
  .component-search { width: 100%; }
  .component-browser { min-height: 0; overflow-y: auto; }
  .browser-component {
    position: relative;
    min-width: 0;
    display: grid;
    grid-template-columns: auto minmax(80px, 1fr);
    gap: 6px;
    align-items: center;
    padding: 6px;
    border: 1px solid var(--accent);
    background: var(--bg-elevated);
  }
  .component-copy { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .component-type { color: var(--fg-dim); font-size: 11px; line-height: 1.3; }
  .component-type span { font-size: 9px; letter-spacing: .45px; text-transform: uppercase; }
  fieldset { min-width: 0; border: 1px solid var(--border); display: flex; flex-direction: column; gap: 7px; }
  legend { color: var(--fg-bright); }
  input[type="number"] { width: 72px; }
  .destination { position: relative; min-height: 52px; display: flex; flex-direction: column; gap: 5px; padding: 6px; border: 1px dashed var(--border); }
  .main-destination { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
  .component { position: relative; min-width: 0; display: grid; grid-template-columns: auto minmax(80px, 1fr) 72px auto; gap: 6px; align-items: center; padding: 6px; border: 1px solid var(--border); background: var(--bg-elevated); }
  .main-destination .component { grid-template-columns: auto minmax(70px, 1fr) auto; }
  .drop-before::before, .drop-after::after, .destination.drop-at-end::after {
    content: "";
    position: absolute;
    left: -3px;
    right: -3px;
    height: 4px;
    z-index: 3;
    border-radius: 2px;
    background: var(--accent);
    box-shadow: 0 0 0 1px var(--bg), 0 0 8px color-mix(in srgb, var(--accent) 75%, transparent);
    pointer-events: none;
  }
  .drop-before::before { top: -5px; }
  .drop-after::after { bottom: -5px; }
  .destination.drop-at-end::after { bottom: 3px; left: 4px; right: 4px; }
  .grip { cursor: grab; color: var(--accent); }
  .danger { flex: 0 0 auto; }
  .empty { padding: 30px; text-align: center; }
  @media (max-width: 900px) {
    .layout-editor { grid-template-columns: minmax(0, 1fr); }
    .status, .main-section, .sidebars-section, .bench-section { grid-column: 1; }
    .status { grid-row: 1; }
    .main-section { grid-row: 2; }
    .sidebars-section { grid-row: 3; }
    .bench-section { grid-row: 4; position: static; }
    .component-browser { max-height: 320px; }
  }
</style>
