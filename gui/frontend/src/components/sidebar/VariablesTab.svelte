<script lang="ts">
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";

  interface Row {
    id: number;
    name: string;
    value: string;
  }

  let nextId = 1;
  let rows = $state<Row[]>(
    Object.entries(store.config?.Commands?.Variables ?? {})
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([name, value]) => ({ id: nextId++, name, value })),
  );
  let dirty = $state(false);
  let saving = $state(false);

  const namePattern = /^[A-Za-z_][A-Za-z0-9_]*$/;
  const validation = $derived.by(() => {
    const seen = new Set<string>();
    for (const row of rows) {
      const name = row.name.trim();
      if (!name && !row.value) continue;
      if (!namePattern.test(name)) return "Names must begin with a letter or _ and contain only letters, numbers, and _.";
      if (seen.has(name)) return `Duplicate variable: ${name}`;
      seen.add(name);
    }
    return "";
  });

  function add() {
    rows = [...rows, { id: nextId++, name: "", value: "" }];
    dirty = true;
  }

  function remove(id: number) {
    rows = rows.filter((row) => row.id !== id);
    dirty = true;
  }

  async function save() {
    if (saving || validation) return;
    const variables: Record<string, string> = {};
    for (const row of rows) {
      const name = row.name.trim();
      if (name) variables[name] = row.value;
    }
    saving = true;
    try {
      await api.setInputVariables(variables);
      if (store.config?.Commands) store.config.Commands.Variables = variables;
      dirty = false;
      store.addToast("Variables", "Saved");
    } catch (e) {
      store.addToast("Save failed", String(e));
    } finally {
      saving = false;
    }
  }
</script>

<div class="toolbar">
  <button class="add" onclick={add} title="Add variable" aria-label="Add variable" tabindex="-1">+</button>
  <button class="save" onclick={save} disabled={!dirty || !!validation || saving} tabindex="-1">
    {saving ? "Saving…" : "Save"}
  </button>
</div>

{#if validation}<div class="error">{validation}</div>{/if}

{#if rows.length === 0}
  <div class="empty dim">No variables configured.</div>
{:else}
  <div class="variables">
    {#each rows as row (row.id)}
      <div class="variable">
        <input
          class="name"
          type="text"
          aria-label="Variable name"
          placeholder="name"
          bind:value={row.name}
          oninput={() => (dirty = true)}
        />
        <span class="separator" aria-hidden="true">:</span>
        <input
          class="value"
          type="text"
          aria-label={`Value for ${row.name || "variable"}`}
          placeholder="value"
          bind:value={row.value}
          oninput={() => (dirty = true)}
          onkeydown={(e) => {
            if (e.key === "Enter" && !validation) {
              e.preventDefault();
              void save();
            }
          }}
        />
        <button class="danger remove" onclick={() => remove(row.id)} title="Remove variable" aria-label="Remove variable" tabindex="-1">✕</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toolbar {
    display: flex;
    justify-content: space-between;
    gap: 6px;
    margin-bottom: 7px;
  }
  .add {
    width: 28px;
    padding: 4px 0;
  }
  .save {
    padding: 4px 10px;
    font-size: 12px;
  }
  .empty,
  .error {
    font-size: 11px;
    line-height: 1.35;
    margin-bottom: 8px;
  }
  .error {
    color: var(--danger, #e06c75);
  }
  .variables {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }
  .variable {
    display: grid;
    grid-template-columns: minmax(0, 2fr) auto minmax(0, 3fr) 24px;
    align-items: center;
    gap: 4px;
  }
  .variable input {
    min-width: 0;
    font: 12px var(--mono);
    padding: 5px 6px;
  }
  .separator {
    color: var(--fg-dim);
    font: 13px var(--mono);
  }
  .remove {
    padding: 0;
    font-size: 11px;
  }
</style>
