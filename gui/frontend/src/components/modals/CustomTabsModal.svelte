<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { CustomTabConfig } from "../../lib/types";

  function clone(t: CustomTabConfig): CustomTabConfig {
    return { ...t, Rules: (t.Rules ?? []).map((r) => ({ ...r })) };
  }

  let tabs = $state<CustomTabConfig[]>((store.config?.UI?.CustomTabs ?? []).map(clone));
  let sel = $state(0);
  let newTabName = $state("");

  const current = $derived(tabs[sel]);

  function addTab() {
    const n = newTabName.trim();
    if (!n) return;
    tabs = [...tabs, { Name: n, Visible: true, EchoCommands: false, Rules: [] }];
    sel = tabs.length - 1;
    newTabName = "";
  }
  function removeTab(i: number) {
    tabs = tabs.filter((_, idx) => idx !== i);
    if (sel >= tabs.length) sel = Math.max(0, tabs.length - 1);
  }
  function addRule() {
    if (!current) return;
    current.Rules = [...current.Rules, { Pattern: "", Include: true, Active: true }];
  }
  function removeRule(i: number) {
    if (!current) return;
    current.Rules = current.Rules.filter((_, idx) => idx !== i);
  }

  async function save() {
    try {
      await api.setCustomTabs(tabs);
      store.config!.UI.CustomTabs = tabs;
      store.rebuildTabs(tabs);
      store.addToast("Custom tabs", "Saved");
    } catch (e) {
      store.addToast("Save failed", String(e));
    }
    store.openModal = null;
  }
</script>

<Modal title="Custom Tabs" wide back onsave={save}>
  <div class="cols">
    <div class="tablist">
      {#each tabs as t, i (i)}
        <div class="trow" class:active={sel === i}>
          <button class="tname" onclick={() => (sel = i)}>{t.Name}</button>
          <button class="danger sm" onclick={() => removeTab(i)}>✕</button>
        </div>
      {/each}
      <div class="row addtab">
        <input type="text" bind:value={newTabName} placeholder="New tab…"
          onkeydown={(e) => {
            if ((e.key === "Enter" || e.key === "Tab") && newTabName.trim()) {
              e.preventDefault();
              addTab();
            }
          }} />
        <button onclick={addTab}>+</button>
      </div>
    </div>

    <div class="editor">
      {#if current}
        <label class="chk"><input type="checkbox" bind:checked={current.Visible} /> Visible</label>
        <label class="chk"><input type="checkbox" bind:checked={current.EchoCommands} /> Route command echoes (exclude-only tabs)</label>
        <div class="ruleshead dim">Rules</div>
        {#each current.Rules as rule, i (i)}
          <div class="rule">
            <button class="sm inc" onclick={() => (rule.Include = !rule.Include)}>
              {rule.Include ? "matches" : "excludes"}
            </button>
            <input type="text" bind:value={rule.Pattern} placeholder="pattern (* ? wildcards)" />
            <input type="checkbox" bind:checked={rule.Active} title="Active" />
            <button class="danger sm" onclick={() => removeRule(i)}>✕</button>
          </div>
        {/each}
        <button class="sm" onclick={addRule}>+ Add rule</button>
      {:else}
        <div class="dim empty">Add a tab to begin.</div>
      {/if}
    </div>
  </div>
</Modal>

<style>
  .cols {
    display: flex;
    gap: 16px;
    min-height: 260px;
  }
  .tablist {
    width: 180px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    border-right: 1px solid var(--border);
    padding-right: 12px;
  }
  .trow {
    display: flex;
    gap: 4px;
  }
  .trow .tname {
    flex: 1;
    text-align: left;
  }
  .trow.active .tname {
    border-color: var(--accent);
    color: var(--accent);
  }
  .addtab {
    margin-top: 6px;
  }
  .addtab input {
    flex: 1;
    min-width: 0;
  }
  .editor {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  .ruleshead {
    margin-top: 6px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .rule {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .rule input[type="text"] {
    flex: 1;
  }
  .inc {
    min-width: 70px;
  }
  .sm {
    padding: 4px 10px;
    font-size: 12px;
  }
  .empty {
    padding: 20px;
  }
</style>
