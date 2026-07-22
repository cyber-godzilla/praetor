<script lang="ts">
  import type { Tab, Line } from "../lib/store.svelte";
  import { store } from "../lib/store.svelte";
  import type { Segment } from "../lib/types";
  import { applyHighlights, compileHighlights, maskIPs, SEARCH_STYLE } from "../lib/highlight";
  import { matchingLineIds, stepIndex } from "../lib/search";
  import { safeColor } from "../lib/color";
  import {
    followBandPx,
    gapToBottom,
    withinBand,
    nextAutoFollow,
    thumbMetrics,
    scrollDeltaForThumbDrag,
  } from "../lib/scroll";

  let { tab }: { tab: Tab } = $props();

  const highlights = $derived(compileHighlights(store.config?.Highlights));
  const hideIPs = $derived(!!store.config?.UI?.HideIPs);
  const fontSize = $derived(store.config?.UI?.OutputFontSize || 14);

  let viewport: HTMLDivElement;
  // Follow the tail whenever the view sits within a band of the newest line
  // (see lib/scroll.ts). Distance — not scroll direction — decides following, so
  // a burst that momentarily outruns the auto-scroll, or a wheel gesture that
  // lands a few pixels short of the bottom, still counts as "following". Only
  // scrolling up out of the band (or pressing Home) detaches; End/PgDn re-engage.
  let autoFollow = true;
  // Last observed scrollTop, used to tell a user's upward scroll (scrollTop
  // decreases) apart from content growth (scrollHeight grows, scrollTop does
  // not). Kept in sync on programmatic scrolls so those never read as user
  // movement. Home also freezes at the top even when the whole buffer is short
  // enough that the top sits inside the band; setting scrollTop=0 fires a scroll
  // event we swallow via ignoreScroll.
  let lastTop = 0;
  let ignoreScroll = false;

  function bandPx(): number {
    return followBandPx(fontSize);
  }

  // Recompute the follow state from the current scroll position. Re-engage
  // whenever within the band, but only DISENGAGE on a genuine upward scroll —
  // a burst that grows scrollHeight faster than the auto-scroll catches up must
  // not spuriously detach following (which would leave the view stuck behind).
  function onScroll() {
    if (!viewport) return;
    const top = viewport.scrollTop;
    if (ignoreScroll) {
      ignoreScroll = false;
      lastTop = top;
      return;
    }
    autoFollow = nextAutoFollow({
      gapPx: gapToBottom(viewport),
      bandPx: bandPx(),
      top,
      lastTop,
      current: autoFollow,
    });
    lastTop = top;
    sampleMetrics();
  }

  // ---- Custom scrollbar rail --------------------------------------------
  // The pane's native scrollbar is hidden so text keeps full height; an overlaid
  // rail (buttons + draggable thumb) drives scrolling instead. Thumb geometry is
  // pure (lib/scroll.ts); we just sample the DOM metrics into reactive state.
  let trackEl: HTMLDivElement;
  const MIN_THUMB = 24;
  let metrics = $state({ scrollTop: 0, scrollHeight: 0, clientHeight: 0, trackPx: 0 });
  const thumb = $derived(thumbMetrics({ ...metrics, minThumbPx: MIN_THUMB }));

  function sampleMetrics() {
    if (!viewport || !trackEl) return;
    metrics = {
      scrollTop: viewport.scrollTop,
      scrollHeight: viewport.scrollHeight,
      clientHeight: viewport.clientHeight,
      trackPx: trackEl.clientHeight,
    };
  }

  let dragging = false;
  let dragStartY = 0;
  let dragStartScroll = 0;

  function onThumbDown(e: PointerEvent) {
    if (!viewport) return;
    e.preventDefault();
    e.stopPropagation(); // don't let the track's page-click fire
    dragging = true;
    dragStartY = e.clientY;
    dragStartScroll = viewport.scrollTop;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function onThumbMove(e: PointerEvent) {
    if (!dragging || !viewport) return;
    const delta = scrollDeltaForThumbDrag({
      dyPx: e.clientY - dragStartY,
      trackPx: metrics.trackPx,
      thumbPx: thumb.sizePx,
      scrollHeight: metrics.scrollHeight,
      clientHeight: metrics.clientHeight,
    });
    const maxScroll = Math.max(0, metrics.scrollHeight - metrics.clientHeight);
    viewport.scrollTop = Math.min(maxScroll, Math.max(0, dragStartScroll + delta));
  }
  function onThumbUp(e: PointerEvent) {
    dragging = false;
    try {
      (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
    } catch {
      /* capture may already be released */
    }
  }
  // Clicking the track above/below the thumb pages toward the click.
  function onTrackDown(e: PointerEvent) {
    if (!trackEl) return;
    const y = e.clientY - trackEl.getBoundingClientRect().top;
    pageBy(y < thumb.offsetPx ? -1 : 1);
  }

  // Coalesce all appends within a frame into a single scroll-to-bottom.
  let scrollQueued = false;
  function followTail() {
    if (scrollQueued) return;
    scrollQueued = true;
    requestAnimationFrame(() => {
      scrollQueued = false;
      if (viewport && autoFollow) {
        viewport.scrollTop = viewport.scrollHeight;
        lastTop = viewport.scrollTop;
      }
    });
  }

  // Explicit scroll commands shared by the on-screen buttons and the keyboard.
  function toTop() {
    if (!viewport) return;
    autoFollow = false; // freeze: appends never re-engage, only End/scroll does
    // Only swallow a scroll event if one will actually fire (position changes).
    if (viewport.scrollTop !== 0) {
      ignoreScroll = true;
      viewport.scrollTop = 0;
    }
    lastTop = 0;
  }
  function toEnd() {
    if (!viewport) return;
    autoFollow = true;
    viewport.scrollTop = viewport.scrollHeight;
    lastTop = viewport.scrollTop;
  }
  function pageBy(dir: 1 | -1) {
    if (!viewport) return;
    viewport.scrollBy({ top: viewport.clientHeight * 0.85 * dir });
    // A PgDn that lands within the band snaps fully to the bottom (== End).
    if (dir > 0 && withinBand(gapToBottom(viewport), bandPx())) toEnd();
    else onScroll();
  }

  $effect(() => {
    // Touch length so the effect re-runs on append.
    void tab.lines.length;
    if (autoFollow) followTail();
    sampleMetrics();
  });

  // ---- Scrollback search (Ctrl+F) ---------------------------------------
  // GameView toggles store.searchOpen from its capture-phase key handling; the
  // query and match cursor live here. Matches are recomputed against the
  // active tab's rendered text; navigation scrolls the match into view.
  let searchEl: HTMLInputElement | undefined = $state();
  let searchQuery = $state("");
  let searchIdx = $state(-1); // index into searchMatches; -1 = none yet

  const searchMatches = $derived(
    store.searchOpen && searchQuery.trim()
      ? matchingLineIds(
          tab.lines.map((l) => ({ id: l.id, text: textOf(l) })),
          searchQuery,
        )
      : [],
  );
  const currentMatchId = $derived(
    searchIdx >= 0 && searchIdx < searchMatches.length ? searchMatches[searchIdx] : -1,
  );

  // Keep the cursor valid as matches change (appends, trims, tab switches):
  // out-of-range or unset snaps to the newest match.
  $effect(() => {
    const len = searchMatches.length;
    if (len === 0) {
      if (searchIdx !== -1) searchIdx = -1;
    } else if (searchIdx < 0 || searchIdx >= len) {
      searchIdx = len - 1;
    }
  });

  // (Re)focus the search input on open and on repeat Ctrl+F.
  $effect(() => {
    void store.searchFocusRequest;
    if (store.searchOpen) {
      queueMicrotask(() => {
        searchEl?.focus();
        searchEl?.select();
      });
    }
  });

  function textOf(line: Line): string {
    return segsFor(line)
      .map((s) => s.text)
      .join("");
  }

  function scrollToCurrent() {
    const id = searchMatches[searchIdx];
    if (id === undefined || !viewport) return;
    const el = viewport.querySelector(`[data-lid="${id}"]`);
    el?.scrollIntoView({ block: "center" });
    onScroll(); // refresh follow state + rail metrics after the jump
  }

  function onQueryInput(v: string) {
    searchQuery = v;
    searchIdx = searchMatches.length - 1; // restart at the newest match
    scrollToCurrent();
  }

  function searchStep(delta: number) {
    if (searchMatches.length === 0) return;
    searchIdx = stepIndex(searchIdx, delta, searchMatches.length);
    scrollToCurrent();
  }

  function closeSearch() {
    store.searchOpen = false;
    store.focusInputRequest++;
  }

  function onSearchKey(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      searchStep(e.shiftKey ? 1 : -1); // Enter walks older; Shift+Enter newer
    }
    // Escape is handled by GameView's capture-phase handler (closes the bar).
  }

  // Re-anchor to the bottom when the viewport geometry changes (window/sidebar
  // resize, font-size change). Without this, scrollHeight/clientHeight shift
  // while scrollTop stays put, leaving the "followed" view stuck at an offset.
  $effect(() => {
    if (!viewport) return;
    const ro = new ResizeObserver(() => {
      if (autoFollow) followTail();
      sampleMetrics();
    });
    ro.observe(viewport);
    return () => ro.disconnect();
  });

  // Font-size changes reflow line heights without resizing the viewport, so
  // re-follow explicitly when it changes.
  $effect(() => {
    void fontSize;
    if (autoFollow) followTail();
    sampleMetrics();
  });

  // Snap to the bottom and resume following when switching to another tab.
  $effect(() => {
    void tab.name;
    autoFollow = true;
    followTail();
    sampleMetrics();
  });

  function segStyle(s: Segment): string {
    const parts: string[] = [];
    // Sanitize colors before they reach the inline style attribute (defense in
    // depth on top of the Go protocol layer's validation) — see safeColor.
    const color = safeColor(s.color);
    if (color) parts.push(`color:${color}`);
    // Background only — no padding/radius, so a highlight tints the exact
    // character cells like the TUI and doesn't add horizontal space.
    const bg = safeColor(s.bg);
    if (bg) parts.push(`background:${bg}`);
    if (s.bold) parts.push("font-weight:700");
    if (s.italic) parts.push("font-style:italic");
    if (s.underline) parts.push("text-decoration:underline");
    return parts.join(";");
  }

  // Apply frontend render transforms in the TUI order: highlights, then IP
  // masking. (Color words are applied upstream in the Go facade.) An active
  // Ctrl+F query is painted last so search matches win visually.
  function displaySegs(line: Line): Segment[] {
    let segs = applyHighlights(segsFor(line), highlights);
    if (hideIPs) segs = maskIPs(segs);
    const q = searchQuery.trim().toLowerCase();
    if (store.searchOpen && q) {
      segs = applyHighlights(segs, [{ pattern: q, bg: SEARCH_STYLE.bg, fg: SEARCH_STYLE.fg }]);
    }
    return segs;
  }

  function reveal(line: Line) {
    if (line.suppressed) line.suppressed.revealed = !line.suppressed.revealed;
  }

  function segsFor(line: Line): Segment[] {
    if (line.suppressed) {
      const showOrig = line.suppressed.revealed || store.expandAllSuppressed;
      return showOrig ? line.suppressed.original : line.suppressed.placeholder;
    }
    return line.segments;
  }

  // Keyboard scroll controls mirror the on-screen buttons. Home/End override the
  // game input's cursor-home/end (that input is single-line and short, so scroll
  // control wins); PgUp/PgDn page the view, with PgDn-near-bottom acting as End.
  function onWindowKey(e: KeyboardEvent) {
    if (store.openModal || !viewport) return;
    // While the search box has focus, Home/End/PgUp/PgDn edit/navigate there.
    if (searchEl && document.activeElement === searchEl) return;
    switch (e.key) {
      case "PageUp":
        e.preventDefault();
        pageBy(-1);
        break;
      case "PageDown":
        e.preventDefault();
        pageBy(1);
        break;
      case "Home":
        e.preventDefault();
        toTop();
        break;
      case "End":
        e.preventDefault();
        toEnd();
        break;
    }
  }

  function isBlank(line: Line): boolean {
    const s = segsFor(line);
    return s.length === 0 || (s.length === 1 && s[0].text === "" && !s[0].isHR);
  }
</script>

<svelte:window onkeydown={onWindowKey} />

<div class="output">
  {#if store.searchOpen}
    <div class="searchbar">
      <input
        bind:this={searchEl}
        value={searchQuery}
        oninput={(e) => onQueryInput(e.currentTarget.value)}
        onkeydown={onSearchKey}
        placeholder="Find in scrollback…"
        spellcheck="false"
        autocomplete="off"
      />
      <span class="count">{searchMatches.length ? `${searchIdx + 1}/${searchMatches.length}` : "0/0"}</span>
      <button type="button" tabindex="-1" title="Older match (Enter)" onclick={() => searchStep(-1)}>▲</button>
      <button type="button" tabindex="-1" title="Newer match (Shift+Enter)" onclick={() => searchStep(1)}>▼</button>
      <button type="button" tabindex="-1" title="Close (Esc)" onclick={closeSearch}>✕</button>
    </div>
  {/if}
  <div class="pane" bind:this={viewport} onscroll={onScroll} style="font-size:{fontSize}px">
    {#each tab.lines as line (line.id)}
      {#if isBlank(line)}
        <div class="line blank">&nbsp;</div>
      {:else}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="line"
          class:echo={line.isEcho}
          class:suppressed={!!line.suppressed}
          class:search-current={line.id === currentMatchId}
          data-lid={line.id}
          onclick={() => line.suppressed && reveal(line)}
          role={line.suppressed ? "button" : undefined}
          tabindex={line.suppressed ? -1 : undefined}
        >
          {#each displaySegs(line) as seg}
            {#if seg.isHR}
              <hr />
            {:else}<span style={segStyle(seg)}>{seg.text}</span>{/if}
          {/each}
        </div>
      {/if}
    {/each}
  </div>

  <!-- Custom scroll rail overlaying the pane's right edge: Home/PgUp cap the top,
       PgDn/End the bottom, and a draggable thumb runs between. The pane keeps
       full height (native scrollbar hidden), so only the scrollbar is replaced —
       the text area is not compressed. Single arrows page, barred arrows jump. -->
  <div class="rail" class:idle={!thumb.scrollable}>
    <div class="rail-btns">
      <button type="button" tabindex="-1" title="Top (Home)" aria-label="Scroll to top" onclick={toTop}>
        <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="3" y="3" width="10" height="1.7" /><path d="M8 6 L13 13 L3 13 Z" /></svg>
      </button>
      <button type="button" tabindex="-1" title="Page up" aria-label="Page up" onclick={() => pageBy(-1)}>
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 4 L13 11 L3 11 Z" /></svg>
      </button>
    </div>

    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="track" bind:this={trackEl} onpointerdown={onTrackDown}>
      {#if thumb.scrollable}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="thumb"
          style="height:{thumb.sizePx}px; transform:translateY({thumb.offsetPx}px)"
          onpointerdown={onThumbDown}
          onpointermove={onThumbMove}
          onpointerup={onThumbUp}
        ></div>
      {/if}
    </div>

    <div class="rail-btns">
      <button type="button" tabindex="-1" title="Page down" aria-label="Page down" onclick={() => pageBy(1)}>
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 12 L3 5 L13 5 Z" /></svg>
      </button>
      <button type="button" tabindex="-1" title="Bottom (End)" aria-label="Scroll to bottom" onclick={toEnd}>
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 10 L3 3 L13 3 Z" /><rect x="3" y="12" width="10" height="1.7" /></svg>
      </button>
    </div>
  </div>
</div>

<style>
  /* The pane fills the whole area; the rail overlays its right edge. */
  .output {
    flex: 1;
    min-height: 0;
    position: relative;
    display: flex;
  }
  .pane {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    /* Extra right padding clears the overlaid rail so text never runs under it. */
    padding: 8px 24px 8px 12px;
    font-family: var(--mono);
    font-size: 14px;
    line-height: 1.4;
    background: var(--bg);
    user-select: text;
  }
  /* Hide the native scrollbar — the custom rail replaces it. Scrolling via
     wheel/keys still works. */
  .pane {
    scrollbar-width: none;
  }
  .pane::-webkit-scrollbar {
    width: 0;
    height: 0;
  }
  /* Custom scrollbar rail: two button clusters capping a draggable thumb. Faint
     at rest, brightening when the pointer is over the output. */
  .rail {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    width: 18px;
    display: flex;
    flex-direction: column;
    align-items: center;
    opacity: 0.6;
    transition: opacity 0.12s;
  }
  .output:hover .rail {
    opacity: 1;
  }
  .rail.idle {
    opacity: 0.35;
  }
  .rail-btns {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 2px 0;
  }
  .rail button {
    width: 18px;
    height: 18px;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    cursor: pointer;
    transition:
      color 0.12s,
      border-color 0.12s;
  }
  .rail button:hover {
    color: var(--accent);
    border-color: var(--border-bright);
  }
  .rail svg {
    width: 12px;
    height: 12px;
    fill: currentColor;
  }
  /* Track fills the space between the button clusters; the thumb is absolutely
     positioned within it via translateY. */
  .track {
    position: relative;
    flex: 1;
    width: 12px;
    margin: 2px 0;
  }
  .thumb {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    background: var(--border);
    cursor: grab;
  }
  .thumb:hover {
    background: var(--border-bright);
  }
  .thumb:active {
    cursor: grabbing;
  }
  /* Ctrl+F search bar overlays the top-right corner, clear of the rail. */
  .searchbar {
    position: absolute;
    top: 6px;
    right: 26px;
    z-index: 2;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 6px;
    background: var(--bg-elevated);
    border: 1px solid var(--border-bright);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35);
  }
  .searchbar input {
    width: 200px;
    font-family: var(--mono);
    font-size: 13px;
    background: var(--bg-input);
    border: 1px solid var(--border);
  }
  .searchbar .count {
    min-width: 44px;
    text-align: center;
    font-family: var(--mono);
    font-size: 11px;
    color: var(--fg-dim);
  }
  .searchbar button {
    width: 22px;
    height: 22px;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    cursor: pointer;
  }
  .searchbar button:hover {
    color: var(--accent);
    border-color: var(--border-bright);
  }
  .line {
    white-space: pre-wrap;
    word-break: break-word;
    min-height: 1.4em;
  }
  .line.search-current {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }
  .line.echo {
    color: var(--fg-dim);
  }
  .line.suppressed {
    cursor: pointer;
  }
  .line.blank {
    min-height: 1.4em;
  }
  hr {
    border: none;
    border-top: 1px solid var(--border-bright);
    margin: 4px 0;
  }
</style>
