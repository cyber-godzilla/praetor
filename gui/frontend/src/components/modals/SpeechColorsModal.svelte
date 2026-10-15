<script lang="ts">
  import { onMount, tick } from "svelte";
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { ColorwordStyle, SpeechColorConfig } from "../../lib/types";
  import { safeColor } from "../../lib/color";
  import { groupColorwordStyles, searchColorwordStyles } from "../../lib/colorword-search";

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
  let openIndex = $state<number | null>(null);
  let query = $state("");
  let activeResult = $state(0);
  let error = $state("");
  let searchInput = $state<HTMLInputElement>();
  let popup = $state<HTMLDivElement>();
  let resultsEl = $state<HTMLDivElement>();
  let anchor = $state<HTMLButtonElement | null>(null);
  let popupLeft = $state(0);
  let popupTop = $state(0);
  let popupWidth = $state(440);

  const visibleStyles = $derived(searchColorwordStyles(store.colorwordStyles, query));
  const groupedStyles = $derived(groupColorwordStyles(visibleStyles));

  function selectedStyle(name: string): ColorwordStyle | undefined {
    return store.colorwordStyles.find((style) => style.name === name);
  }

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
    if (openIndex === index) closePicker();
    items = items.filter((_, itemIndex) => itemIndex !== index);
  }

  function toggle(index: number) {
    items[index].Active = !items[index].Active;
  }

  function placePopup() {
    if (!anchor || openIndex === null) return;
    const rect = anchor.getBoundingClientRect();
    const margin = 10;
    popupWidth = Math.min(480, window.innerWidth - margin * 2);
    popupLeft = Math.max(margin, Math.min(rect.left, window.innerWidth - popupWidth - margin));

    const estimatedHeight = 350;
    const below = window.innerHeight - rect.bottom - margin;
    const above = rect.top - margin;
    popupTop = below >= Math.min(estimatedHeight, above)
      ? rect.bottom + 4
      : Math.max(margin, rect.top - Math.min(estimatedHeight, above) - 4);
  }

  async function openPicker(index: number, target: HTMLButtonElement) {
    openIndex = index;
    anchor = target;
    query = "";
    activeResult = Math.max(0, visibleStyles.findIndex((style) => style.name === items[index].Style));
    placePopup();
    await tick();
    placePopup();
    searchInput?.focus();
  }

  function closePicker() {
    openIndex = null;
    anchor = null;
    query = "";
  }

  function choose(style: ColorwordStyle) {
    if (openIndex === null) return;
    items[openIndex].Style = style.name;
    closePicker();
  }

  async function showActiveResult() {
    await tick();
    resultsEl?.querySelector<HTMLElement>(`[data-result-index="${activeResult}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }

  function searchKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      const focusTarget = anchor;
      closePicker();
      focusTarget?.focus();
      return;
    }
    if (event.key === "Enter" && visibleStyles[activeResult]) {
      event.preventDefault();
      choose(visibleStyles[activeResult]);
      return;
    }
    let next = activeResult;
    if (event.key === "ArrowDown") next++;
    else if (event.key === "ArrowUp") next--;
    else if (event.key === "PageDown") next += 6;
    else if (event.key === "PageUp") next -= 6;
    else return;
    event.preventDefault();
    activeResult = Math.max(0, Math.min(visibleStyles.length - 1, next));
    void showActiveResult();
  }

  function searchChanged() {
    activeResult = 0;
    void showActiveResult();
  }

  function previewCharacters(style: ColorwordStyle): { character: string; color: string }[] {
    let colorIndex = 0;
    return [...style.label].map((character) => {
      const color = safeColor(style.colors[colorIndex % style.colors.length]);
      if (character !== " " && character !== "-") colorIndex++;
      return { character, color };
    });
  }

  function swatchBackground(style: ColorwordStyle): string {
    const colors = style.colors.map(safeColor).filter(Boolean);
    if (colors.length < 2) return colors[0] ?? "transparent";
    return `linear-gradient(90deg, ${colors.join(", ")})`;
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

  onMount(() => {
    const dismiss = (event: PointerEvent) => {
      const target = event.target as Node;
      if (openIndex !== null && !popup?.contains(target) && !anchor?.contains(target)) closePicker();
    };
    const reposition = () => placePopup();
    window.addEventListener("pointerdown", dismiss, true);
    window.addEventListener("resize", reposition);
    window.addEventListener("scroll", reposition, true);
    return () => {
      window.removeEventListener("pointerdown", dismiss, true);
      window.removeEventListener("resize", reposition);
      window.removeEventListener("scroll", reposition, true);
    };
  });
</script>

<Modal title="Speech Colors" wide back onsave={save} onclose={closePicker}>
  <p class="hint dim">
    Match an entire speaker name; use * and ? wildcards or {"${name}"} variables.
    Think and OOC lines can be included separately. Color words within the line still keep their own colors.
  </p>

  <div class="head" aria-hidden="true">
    <span>On</span><span>Character pattern</span><span>Colorword style</span><span>Also color</span><span></span>
  </div>
  <div class="rules">
    {#each items as item, index (index)}
      {@const style = selectedStyle(item.Style)}
      <div class="rule" class:off={!item.Active}>
        <input
          type="checkbox"
          checked={item.Active}
          onchange={() => toggle(index)}
          aria-label={`Enable speech color for ${item.Character || "new character"}`}
        />
        <input
          type="text"
          bind:value={item.Character}
          data-character-index={index}
          placeholder="Name, wildcard, or ${name}"
          aria-label="Character pattern"
          oninput={() => (error = "")}
        />
        <button
          class="style-choice"
          type="button"
          aria-haspopup="listbox"
          aria-expanded={openIndex === index}
          onclick={(event) => openPicker(index, event.currentTarget)}
        >
          {#if style}
            <span class="swatch" style:background={swatchBackground(style)}></span>
            <span class="choice-name">
              {#each previewCharacters(style) as token, tokenIndex (tokenIndex)}
                <span style:color={token.color}>{token.character}</span>
              {/each}
            </span>
            <span class="family">{style.family}</span>
          {:else}
            <span class="choice-name">{item.Style}</span>
          {/if}
          <span class="chevron">▾</span>
        </button>
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

{#if openIndex !== null}
  <div
    class="picker"
    bind:this={popup}
    style={`left:${popupLeft}px;top:${popupTop}px;width:${popupWidth}px`}
    role="dialog"
    aria-label="Choose a Colorword style"
  >
    <div class="search-wrap">
      <span class="search-icon" aria-hidden="true">⌕</span>
      <input
        bind:this={searchInput}
        bind:value={query}
        type="text"
        role="combobox"
        aria-label="Search color names or families"
        aria-expanded="true"
        aria-controls="speech-color-results"
        placeholder="Search a color or family…"
        oninput={searchChanged}
        onkeydown={searchKeydown}
      />
      {#if query}<button class="clear" onclick={() => { query = ""; searchChanged(); }} aria-label="Clear search">✕</button>{/if}
    </div>
    <div class="search-note">Try “crim” for Crimson or “red” for the whole Reds family.</div>
    <div class="results" id="speech-color-results" bind:this={resultsEl} role="listbox">
      {#each groupedStyles as group (group.family)}
        <div class="group-label">{group.family}</div>
        <div class="cards">
          {#each group.styles as style (style.name)}
            {@const resultIndex = visibleStyles.indexOf(style)}
            <button
              class="style-card"
              class:active={resultIndex === activeResult}
              class:selected={items[openIndex]?.Style === style.name}
              data-result-index={resultIndex}
              role="option"
              aria-selected={items[openIndex]?.Style === style.name}
              onmouseenter={() => (activeResult = resultIndex)}
              onclick={() => choose(style)}
            >
              <span class="card-swatch" style:background={swatchBackground(style)}></span>
              <span class="card-copy">
                <span class="card-name">
                  {#each previewCharacters(style) as token, tokenIndex (tokenIndex)}
                    <span style:color={token.color}>{token.character}</span>
                  {/each}
                </span>
                {#if style.kind !== "solid"}<span class="special">{style.kind}</span>{/if}
              </span>
              {#if items[openIndex]?.Style === style.name}<span class="check">✓</span>{/if}
            </button>
          {/each}
        </div>
      {/each}
      {#if visibleStyles.length === 0}
        <div class="empty dim">No Colorword styles match “{query}”.</div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .hint { margin: 0 0 15px; font-size: 12px; line-height: 1.5; }
  .head, .rule { display: grid; grid-template-columns: 28px minmax(120px, .8fr) minmax(190px, 1.2fr) 126px 34px; gap: 8px; align-items: center; }
  .head { padding: 0 7px 5px; color: var(--fg-dim); font-size: 10px; letter-spacing: .08em; text-transform: uppercase; }
  .rules { display: flex; flex-direction: column; gap: 7px; margin-bottom: 12px; }
  .rule { padding: 7px; border: 1px solid var(--border); background: color-mix(in srgb, var(--bg-input) 65%, transparent); }
  .rule.off { opacity: .55; }
  .rule input[type="text"] { min-width: 0; width: 100%; box-sizing: border-box; }
  .style-choice { min-width: 0; display: grid; grid-template-columns: 22px minmax(60px, 1fr) auto auto; align-items: center; gap: 8px; padding: 5px 8px; text-align: left; background: var(--bg-input); }
  .channels { display: flex; gap: 5px; }
  .channels label { display: flex; align-items: center; gap: 4px; min-width: 0; color: var(--fg-dim); font-size: 10px; cursor: pointer; }
  .channels input { margin: 0; }
  .swatch, .card-swatch { border: 1px solid rgba(255,255,255,.3); box-shadow: inset 0 0 0 1px rgba(0,0,0,.35); }
  .swatch { width: 18px; height: 12px; }
  .choice-name, .card-name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
  .family { color: var(--fg-dim); font-size: 10px; text-transform: uppercase; }
  .chevron { color: var(--accent); }
  .remove { min-width: 0; padding: 5px 8px; }
  .add { width: 100%; border-style: dashed; }
  .error { margin: 0 0 10px; color: var(--red); font-size: 12px; }
  .empty { padding: 16px; text-align: center; border: 1px dashed var(--border); }

  .picker { position: fixed; z-index: 550; max-height: calc(100vh - 20px); display: flex; flex-direction: column; box-sizing: border-box; background: var(--bg-panel); border: 1px solid var(--accent); box-shadow: 0 12px 36px rgba(0,0,0,.65); }
  .search-wrap { position: relative; display: flex; align-items: center; padding: 9px 9px 5px; }
  .search-wrap input { width: 100%; box-sizing: border-box; padding-left: 29px; padding-right: 30px; }
  .search-icon { position: absolute; left: 18px; color: var(--accent); pointer-events: none; }
  .clear { position: absolute; right: 13px; min-width: 0; padding: 2px 7px; border: 0; color: var(--fg-dim); }
  .search-note { padding: 0 11px 8px; color: var(--fg-dim); font-size: 10px; }
  .results { max-height: 258px; overflow-y: scroll; overscroll-behavior: contain; scrollbar-gutter: stable; border-top: 1px solid var(--border); padding: 0 7px 8px; }
  .group-label { position: sticky; top: 0; z-index: 1; padding: 8px 4px 5px; color: var(--accent); background: var(--bg-panel); font-size: 10px; letter-spacing: .09em; text-transform: uppercase; }
  .cards { display: grid; grid-template-columns: 1fr 1fr; gap: 5px; }
  .style-card { min-width: 0; display: flex; align-items: center; gap: 8px; padding: 7px 8px; text-align: left; background: var(--bg-input); }
  .style-card.active { border-color: var(--border-bright); }
  .style-card.selected { border-color: var(--accent); background: color-mix(in srgb, var(--accent) 8%, var(--bg-input)); }
  .card-swatch { width: 25px; height: 15px; flex: 0 0 auto; }
  .card-copy { min-width: 0; display: flex; flex-direction: column; flex: 1; }
  .card-name { font-size: 12px; }
  .special { color: var(--fg-dim); font-size: 9px; text-transform: uppercase; }
  .check { color: var(--accent); }

  @media (max-width: 620px) {
    .head { display: none; }
    .rule { grid-template-columns: 22px 1fr 34px; }
    .rule input[type="text"] { grid-column: 2 / 3; }
    .style-choice { grid-column: 2 / 3; }
    .channels { grid-column: 2 / 3; }
    .remove { grid-column: 3; grid-row: 1; }
    .cards { grid-template-columns: 1fr; }
    .family { display: none; }
  }
</style>
