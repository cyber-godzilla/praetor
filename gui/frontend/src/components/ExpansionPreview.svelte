<script lang="ts">
  import { store } from "../lib/store.svelte";
  import { expandVariables, VariableExpansionError } from "../lib/variables";

  let { value, match = false }: { value: string; match?: boolean } = $props();

  const preview = $derived.by(() => {
    if (!value.includes("${") && !value.includes("\\${")) return null;
    try {
      const resolved = expandVariables(value, store.resolvedVariables());
      return {
        error: false,
        text: match && resolved === "" ? "Resolves empty — matches nothing" : `Resolves to: ${resolved || "(empty)"}`,
      };
    } catch (error) {
      const name = error instanceof VariableExpansionError && error.variable
        ? ` ${error.variable}`
        : "";
      return { error: true, text: `Unresolved${name}: ${error instanceof Error ? error.message : String(error)}` };
    }
  });
</script>

{#if preview}
  <span class="expansion" class:error={preview.error}>{preview.text}</span>
{/if}

<style>
  .expansion {
    display: block;
    margin-top: 3px;
    color: var(--fg-dim);
    font-family: var(--mono);
    font-size: 10px;
    overflow-wrap: anywhere;
  }
  .expansion.error { color: var(--red); }
</style>
