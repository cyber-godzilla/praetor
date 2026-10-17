<script lang="ts">
  import { store } from "../lib/store.svelte";
  import OutputPane from "./OutputPane.svelte";
  import MetricsPanel from "./MetricsPanel.svelte";
  import MapView from "./MapView.svelte";
  import ExitsView from "./ExitsView.svelte";
  import VitalsView from "./VitalsView.svelte";
  import ActionsTab from "./sidebar/ActionsTab.svelte";
  import ModesTab from "./sidebar/ModesTab.svelte";
  import VariablesTab from "./sidebar/VariablesTab.svelte";

  let { id, presentation }: { id: string; presentation: "tab" | "sidebar" } = $props();
  const view = $derived(store.viewByID(id));
  const compact = $derived(presentation === "sidebar");
</script>

{#if view?.kind === "metrics"}
  <MetricsPanel {compact} />
{:else if view?.kind === "map"}
  <MapView {compact} />
{:else if view?.kind === "exits"}
  <ExitsView {compact} />
{:else if view?.kind === "vitals"}
  <VitalsView {compact} />
{:else if view?.kind === "actions"}
  <div class="workspace" class:compact><ActionsTab /></div>
{:else if view?.kind === "modes"}
  <div class="workspace" class:compact><ModesTab /></div>
{:else if view?.kind === "variables"}
  <div class="workspace" class:compact><VariablesTab /></div>
{:else if view}
  <OutputPane tab={view} />
{:else}
  <div class="missing dim">View unavailable.</div>
{/if}

<style>
  .workspace { flex: 1; min-height: 0; overflow: auto; padding: 18px; }
  .workspace.compact { padding: 2px; }
  .missing { padding: 20px; }
</style>
