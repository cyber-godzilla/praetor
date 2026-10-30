<script lang="ts">
  import { onMount, tick } from "svelte";
  import { store } from "../lib/store.svelte";
  import type { ColorwordStyle } from "../lib/types";
  import { safeColor } from "../lib/color";
  import { groupColorwordStyles, searchColorwordStyles } from "../lib/colorword-search";

  let {
    value = $bindable(),
    ariaLabel = "Choose a Colorword style",
  }: {
    value: string;
    ariaLabel?: string;
  } = $props();

  let open = $state(false);
  let query = $state("");
  let activeResult = $state(0);
  let anchor = $state<HTMLButtonElement>();
  let popup = $state<HTMLDivElement>();
  let searchInput = $state<HTMLInputElement>();
  let resultsEl = $state<HTMLDivElement>();
  let popupLeft = $state(0);
  let popupTop = $state(0);
  let popupWidth = $state(440);

  const selected = $derived(store.colorwordStyles.find((style) => style.name === value));
  const visibleStyles = $derived(searchColorwordStyles(store.colorwordStyles, query));
  const groupedStyles = $derived(groupColorwordStyles(visibleStyles));

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

  function placePopup() {
    if (!anchor || !open) return;
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

  async function showPicker() {
    open = true;
    query = "";
    activeResult = Math.max(0, store.colorwordStyles.findIndex((style) => style.name === value));
    placePopup();
    await tick();
    placePopup();
    searchInput?.focus();
  }

  function closePicker() {
    open = false;
    query = "";
  }

  function choose(style: ColorwordStyle) {
    value = style.name;
    closePicker();
    anchor?.focus();
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
      closePicker();
      anchor?.focus();
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

  onMount(() => {
    const dismiss = (event: PointerEvent) => {
      const target = event.target as Node;
      if (open && !popup?.contains(target) && !anchor?.contains(target)) closePicker();
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

<button
  bind:this={anchor}
  class="style-choice"
  type="button"
  aria-label={ariaLabel}
  aria-haspopup="listbox"
  aria-expanded={open}
  onclick={showPicker}
>
  {#if selected}
    <span class="swatch" style:background={swatchBackground(selected)}></span>
    <span class="choice-name">
      {#each previewCharacters(selected) as token, tokenIndex (tokenIndex)}
        <span style:color={token.color}>{token.character}</span>
      {/each}
    </span>
    <span class="family">{selected.family}</span>
  {:else}
    <span class="choice-name">{value}</span>
  {/if}
  <span class="chevron">▾</span>
</button>

{#if open}
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
        aria-controls="colorword-style-results"
        placeholder="Search a color or family…"
        oninput={() => { activeResult = 0; void showActiveResult(); }}
        onkeydown={searchKeydown}
      />
      {#if query}<button class="clear" onclick={() => { query = ""; activeResult = 0; }} aria-label="Clear search">✕</button>{/if}
    </div>
    <div class="search-note">Try “crim” for Crimson or “red” for the whole Reds family.</div>
    <div class="results" id="colorword-style-results" bind:this={resultsEl} role="listbox">
      {#each groupedStyles as group (group.family)}
        <div class="group-label">{group.family}</div>
        <div class="cards">
          {#each group.styles as style (style.name)}
            {@const resultIndex = visibleStyles.indexOf(style)}
            <button
              class="style-card"
              class:active={resultIndex === activeResult}
              class:selected={value === style.name}
              data-result-index={resultIndex}
              role="option"
              aria-selected={value === style.name}
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
              {#if value === style.name}<span class="check">✓</span>{/if}
            </button>
          {/each}
        </div>
      {/each}
      {#if visibleStyles.length === 0}<div class="empty dim">No Colorword styles match “{query}”.</div>{/if}
    </div>
  </div>
{/if}

<style>
  .style-choice { width: 100%; min-width: 0; display: grid; grid-template-columns: 22px minmax(60px, 1fr) auto auto; align-items: center; gap: 8px; padding: 5px 8px; text-align: left; background: var(--bg-input); }
  .swatch, .card-swatch { display: block; border: 1px solid color-mix(in srgb, var(--fg) 25%, transparent); }
  .swatch { width: 20px; height: 20px; }
  .choice-name, .card-name { overflow: hidden; font-weight: 700; white-space: nowrap; text-overflow: ellipsis; }
  .family { color: var(--fg-dim); font-size: 9px; text-transform: uppercase; }
  .chevron { color: var(--fg-dim); }
  .picker { position: fixed; z-index: 900; box-sizing: border-box; max-height: min(350px, calc(100vh - 20px)); padding: 10px; overflow: hidden; border: 1px solid var(--accent); background: var(--bg-panel); box-shadow: 0 10px 30px rgba(0, 0, 0, .55); }
  .search-wrap { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 7px; }
  .search-wrap input { min-width: 0; width: 100%; box-sizing: border-box; }
  .search-icon { color: var(--accent); }
  .clear { min-width: 28px; padding-inline: 6px; }
  .search-note { margin: 6px 0 9px 22px; color: var(--fg-dim); font-size: 10px; }
  .results { max-height: 270px; overflow-y: auto; }
  .group-label { position: sticky; top: 0; z-index: 1; padding: 5px 3px; color: var(--accent); background: var(--bg-panel); font-size: 10px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
  .cards { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 5px; margin-bottom: 7px; }
  .style-card { display: grid; grid-template-columns: 28px minmax(0, 1fr) auto; align-items: center; gap: 7px; min-width: 0; padding: 6px; text-align: left; }
  .style-card.active { border-color: var(--accent); }
  .style-card.selected { background: color-mix(in srgb, var(--accent) 12%, var(--bg-input)); }
  .card-swatch { width: 26px; height: 26px; }
  .card-copy { min-width: 0; display: flex; flex-direction: column; }
  .special { color: var(--fg-dim); font-size: 9px; text-transform: uppercase; }
  .check { color: var(--accent); }
  .empty { padding: 18px; text-align: center; }
  @media (max-width: 599px) { .cards { grid-template-columns: 1fr; } .family { display: none; } }
</style>
