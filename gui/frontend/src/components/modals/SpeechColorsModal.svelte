<script lang="ts">
  import { tick } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { SpeechColorConfig } from "../../lib/types";
  import ExpansionPreview from "../ExpansionPreview.svelte";
  import ColorwordStylePicker from "../ColorwordStylePicker.svelte";

  const prefill = store.speechColorPrefill.trim();
  const initialItems = (store.config?.SpeechColors ?? []).map((rule) => ({
    ...rule,
    IncludeThink: rule.IncludeThink ?? false,
    IncludeOOC: rule.IncludeOOC ?? false,
  }));
  if (prefill && !initialItems.some((rule) => rule.Character.toLocaleLowerCase() === prefill.toLocaleLowerCase())) {
    initialItems.push({
      Character: prefill,
      Style: fallbackStyle(),
      Active: true,
      IncludeThink: false,
      IncludeOOC: false,
    });
  }
  store.speechColorPrefill = "";
  let items = $state<SpeechColorConfig[]>(initialItems);
  let error = $state("");

  function fallbackStyle(): string {
    return store.colorwordStyles.find((style) => style.name === "red")?.name
      ?? store.colorwordStyles[0]?.name
      ?? "red";
  }

  async function add() {
    items = [...items, {
      Character: "",
      Style: fallbackStyle(),
      Active: true,
      IncludeThink: false,
      IncludeOOC: false,
    }];
    error = "";
    await tick();
    document.querySelector<HTMLInputElement>(`[data-character-index="${items.length - 1}"]`)?.focus();
  }

  function remove(index: number) {
    items = items.filter((_, itemIndex) => itemIndex !== index);
  }

  function toggle(index: number) {
    items[index].Active = !items[index].Active;
  }

  async function save() {
    const clean = items.map((item) => ({
      Character: item.Character.trim(),
      Style: item.Style,
      Active: item.Active,
      IncludeThink: item.IncludeThink,
      IncludeOOC: item.IncludeOOC,
    }));
    if (clean.some((item) => !item.Character)) {
      error = "Every rule needs a character pattern.";
      return;
    }
    const names = clean.map((item) => item.Character.toLocaleLowerCase());
    if (new Set(names).size !== names.length) {
      error = "Each character pattern can only have one speech color.";
      return;
    }
    try {
      await api.setSpeechColors(clean);
      store.config!.SpeechColors = clean;
      store.addToast("Speech Colors", "Saved");
      store.openModal = null;
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : String(saveError);
    }
  }
</script>

<Modal title="Speech Colors" wide back onsave={save}>
  <p class="hint dim">
    Match an entire speaker name; use * and ? wildcards or {"${name}"} variables.
    Think and OOC lines can be included separately. Color words within the line still keep their own colors.
  </p>

  <div class="head" aria-hidden="true">
    <span>On</span><span>Character pattern</span><span>Colorword style</span><span>Also color</span><span></span>
  </div>
  <div class="rules">
    {#each items as item, index (index)}
      <div class="rule" class:off={!item.Active}>
        <input
          type="checkbox"
          checked={item.Active}
          onchange={() => toggle(index)}
          aria-label={`Enable speech color for ${item.Character || "new character"}`}
        />
        <span class="character-pattern">
          <input
            type="text"
            bind:value={item.Character}
            data-character-index={index}
            placeholder={'Name, wildcard, or ${name}'}
            aria-label="Character pattern"
            oninput={() => (error = "")}
          />
          <ExpansionPreview value={item.Character} match />
        </span>
        <span class="style-picker"><ColorwordStylePicker bind:value={item.Style} ariaLabel={`Choose speech color for ${item.Character || "new character"}`} /></span>
        <div class="channels">
          <label>
            <input
              type="checkbox"
              bind:checked={item.IncludeThink}
              aria-label={`Include think for ${item.Character || "new character"}`}
            />
            Think
          </label>
          <label>
            <input
              type="checkbox"
              bind:checked={item.IncludeOOC}
              aria-label={`Include OOC for ${item.Character || "new character"}`}
            />
            OOC
          </label>
        </div>
        <button class="danger remove" onclick={() => remove(index)} aria-label="Remove rule">✕</button>
      </div>
    {/each}
    {#if items.length === 0}
      <div class="empty dim">No speech colors configured.</div>
    {/if}
  </div>

  {#if error}<div class="error" role="alert">{error}</div>{/if}
  <button class="add" onclick={add}>+ Add character</button>
</Modal>

<style>
  .hint { margin: 0 0 15px; font-size: 12px; line-height: 1.5; }
  .head, .rule { display: grid; grid-template-columns: 28px minmax(120px, .8fr) minmax(190px, 1.2fr) 126px 34px; gap: 8px; align-items: center; }
  .head { padding: 0 7px 5px; color: var(--fg-dim); font-size: 10px; letter-spacing: .08em; text-transform: uppercase; }
  .rules { display: flex; flex-direction: column; gap: 7px; margin-bottom: 12px; }
  .rule { padding: 7px; border: 1px solid var(--border); background: color-mix(in srgb, var(--bg-input) 65%, transparent); }
  .rule.off { opacity: .55; }
  .rule input[type="text"] { min-width: 0; width: 100%; box-sizing: border-box; }
  .character-pattern { min-width: 0; }
  .style-picker { min-width: 0; }
  .channels { display: flex; gap: 5px; }
  .channels label { display: flex; align-items: center; gap: 4px; min-width: 0; color: var(--fg-dim); font-size: 10px; cursor: pointer; }
  .channels input { margin: 0; }
  .remove { min-width: 0; padding: 5px 8px; }
  .add { width: 100%; border-style: dashed; }
  .error { margin: 0 0 10px; color: var(--red); font-size: 12px; }
  .empty { padding: 16px; text-align: center; border: 1px dashed var(--border); }
  @media (max-width: 620px) {
    .head { display: none; }
    .rule { grid-template-columns: 22px 1fr 34px; }
    .rule input[type="text"] { grid-column: 2 / 3; }
    .style-picker { grid-column: 2 / 3; }
    .channels { grid-column: 2 / 3; }
    .remove { grid-column: 3; grid-row: 1; }
  }
</style>
