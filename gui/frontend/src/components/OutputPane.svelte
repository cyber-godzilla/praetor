<script lang="ts">
  import { onDestroy, tick } from "svelte";
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
  let contentEl: HTMLDivElement;
  // Follow the tail whenever the view sits within a band of the newest line
  // (see lib/scroll.ts). Distance — not scroll direction — decides following, so
  // a burst that momentarily outruns the auto-scroll, or a wheel gesture that
  // lands a few pixels short of the bottom, still counts as "following". Only
  // scrolling up out of the band (or pressing Home) detaches; End/PgDn re-engage.
  let autoFollow = $state(true);
  // Scroll events also come from DOM anchoring and programmatic movement. Only
  // a short-lived explicit wheel/touch/control gesture may disengage following;
  // capped scrollback removing rows above the viewport must not look like one.
  const USER_SCROLL_IDLE_MS = 180;
  let userScrollActive = false;
  let userScrollTimer: ReturnType<typeof setTimeout> | undefined;
  let lastTouchY: number | undefined;
  let ignoreScroll = false;

  function bandPx(): number {
    return followBandPx(fontSize);
  }

  function endUserScroll() {
    userScrollActive = false;
    if (userScrollTimer !== undefined) clearTimeout(userScrollTimer);
    userScrollTimer = undefined;
  }

  function markUserScroll() {
    userScrollActive = true;
    if (userScrollTimer !== undefined) clearTimeout(userScrollTimer);
    userScrollTimer = setTimeout(endUserScroll, USER_SCROLL_IDLE_MS);
  }

  // Recompute the follow state from the current scroll position. Application
  // layout may re-engage near the tail but cannot detach it; only an active user
  // gesture is allowed to take ownership of scrollback.
  function onScroll() {
    if (!viewport) return;
    if (ignoreScroll) {
      ignoreScroll = false;
      sampleMetrics();
      return;
    }
    autoFollow = nextAutoFollow({
      gapPx: gapToBottom(viewport),
      bandPx: bandPx(),
      current: autoFollow,
      userMovedAway: userScrollActive,
    });
    // Native wheel/touch momentum can continue after its input event. Keep the
    // user-intent window alive until scrolling itself has gone quiet.
    if (userScrollActive) markUserScroll();
    sampleMetrics();
  }

  function onViewportWheel(e: WheelEvent) {
    if (e.deltaY < 0) markUserScroll();
    else if (e.deltaY > 0) endUserScroll();
  }

  function onViewportTouchStart(e: TouchEvent) {
    lastTouchY = e.touches[0]?.clientY;
  }

  function onViewportTouchMove(e: TouchEvent) {
    const y = e.touches[0]?.clientY;
    if (y === undefined) return;
    if (lastTouchY !== undefined) {
      // Dragging the finger downward moves the viewport toward older output.
      if (y > lastTouchY) markUserScroll();
      else if (y < lastTouchY) endUserScroll();
    }
    lastTouchY = y;
  }

  function onViewportTouchEnd() {
    lastTouchY = undefined;
  }

  function applyUserScrollPosition(movedAway: boolean) {
    if (!viewport) return;
    autoFollow = nextAutoFollow({
      gapPx: gapToBottom(viewport),
      bandPx: bandPx(),
      current: autoFollow,
      userMovedAway: movedAway,
    });
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
    const previousTop = viewport.scrollTop;
    const delta = scrollDeltaForThumbDrag({
      dyPx: e.clientY - dragStartY,
      trackPx: metrics.trackPx,
      thumbPx: thumb.sizePx,
      scrollHeight: metrics.scrollHeight,
      clientHeight: metrics.clientHeight,
    });
    const maxScroll = Math.max(0, metrics.scrollHeight - metrics.clientHeight);
    viewport.scrollTop = Math.min(maxScroll, Math.max(0, dragStartScroll + delta));
    const movedAway = viewport.scrollTop < previousTop;
    if (movedAway) markUserScroll();
    else endUserScroll();
    applyUserScrollPosition(movedAway);
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

  // Coalesce appends, wait for Svelte's DOM update, then anchor and verify on
  // consecutive frames. The bounded verification covers wrapping/layout that
  // settles after the first scroll without creating an unbounded RAF loop.
  const TAIL_TOLERANCE_PX = 1;
  let tailScheduled = false;
  let tailFrame = 0;
  let verifyFrame = 0;
  let destroyed = false;

  function setTailPosition() {
    if (!viewport || !autoFollow) return;
    viewport.scrollTop = viewport.scrollHeight;
    sampleMetrics();
  }

  function queueTailVerification() {
    if (verifyFrame || destroyed) return;
    verifyFrame = requestAnimationFrame(() => {
      verifyFrame = 0;
      if (!viewport || !autoFollow) return;
      if (gapToBottom(viewport) > TAIL_TOLERANCE_PX) {
        viewport.scrollTop = viewport.scrollHeight;
      }
      sampleMetrics();
    });
  }

  function followTail(immediate = false) {
    if (immediate) setTailPosition();
    if (tailScheduled || destroyed) return;
    tailScheduled = true;
    void tick().then(() => {
      if (destroyed) {
        tailScheduled = false;
        return;
      }
      tailFrame = requestAnimationFrame(() => {
        tailFrame = 0;
        tailScheduled = false;
        if (!viewport || !autoFollow) return;
        setTailPosition();
        queueTailVerification();
      });
    });
  }

  // Explicit scroll commands shared by the on-screen buttons and the keyboard.
  function toTop() {
    if (!viewport) return;
    endUserScroll();
    autoFollow = false; // freeze: appends never re-engage, only End/scroll does
    // Only swallow a scroll event if one will actually fire (position changes).
    if (viewport.scrollTop !== 0) {
      ignoreScroll = true;
      viewport.scrollTop = 0;
    }
    sampleMetrics();
  }
  function toEnd() {
    if (!viewport) return;
    endUserScroll();
    ignoreScroll = false;
    autoFollow = true;
    // Move immediately for responsive controls, then repeat after pending DOM
    // and layout work so End cannot target a stale scrollHeight.
    followTail(true);
  }
  function pageBy(dir: 1 | -1) {
    if (!viewport) return;
    const movedAway = dir < 0;
    if (movedAway) markUserScroll();
    else endUserScroll();
    viewport.scrollBy({ top: viewport.clientHeight * 0.85 * dir });
    // A PgDn that lands within the band snaps fully to the bottom (== End).
    if (dir > 0 && withinBand(gapToBottom(viewport), bandPx())) toEnd();
    else applyUserScrollPosition(movedAway);
  }

  $effect(() => {
    // At the scrollback cap, length returns to the same value after every head
    // trim. Also depend on the newest identity so every append schedules follow.
    void tab.lines.length;
    void tab.lines[tab.lines.length - 1]?.id;
    if (autoFollow) followTail();
    sampleMetrics();
  });

  // Re-anchor when either the viewport or rendered content changes geometry.
  // Observing only the fixed viewport misses child insertion and line wrapping.
  $effect(() => {
    if (!viewport || !contentEl) return;
    const ro = new ResizeObserver(() => {
      if (autoFollow) followTail();
      sampleMetrics();
    });
    ro.observe(viewport);
    ro.observe(contentEl);
    return () => ro.disconnect();
  });

  // Font-size changes reflow line heights without resizing the viewport, so
  // re-follow explicitly when it changes.
  $effect(() => {
    void fontSize;
    if (autoFollow) followTail();
    sampleMetrics();
  });

  onDestroy(() => {
    destroyed = true;
    endUserScroll();
    if (tailFrame) cancelAnimationFrame(tailFrame);
    if (verifyFrame) cancelAnimationFrame(verifyFrame);
  });

  // Snap to the bottom and resume following when switching to another tab.
  $effect(() => {
    void tab.name;
    autoFollow = true;
    followTail();
    sampleMetrics();
  });

  // ---- Scrollback search (Ctrl+F) ---------------------------------------
  let searchEl: HTMLInputElement | undefined = $state();
  let searchQuery = $state("");
  let searchIdx = $state(-1);

  const searchMatches = $derived(
    store.searchOpen && searchQuery.trim()
      ? matchingLineIds(
          tab.lines.map((line) => ({ id: line.id, text: textOf(line) })),
          searchQuery,
        )
      : [],
  );
  const currentMatchId = $derived(
    searchIdx >= 0 && searchIdx < searchMatches.length ? searchMatches[searchIdx] : -1,
  );

  $effect(() => {
    const len = searchMatches.length;
    if (len === 0) {
      if (searchIdx !== -1) searchIdx = -1;
    } else if (searchIdx < 0 || searchIdx >= len) {
      searchIdx = len - 1;
    }
  });

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
      .map((segment) => segment.text)
      .join("");
  }

  function scrollToCurrent() {
    const id = searchMatches[searchIdx];
    if (id === undefined || !viewport) return;
    const element = viewport.querySelector(`[data-lid="${id}"]`);
    element?.scrollIntoView({ block: "center" });
    onScroll();
  }

  function onQueryInput(query: string) {
    searchQuery = query;
    searchIdx = searchMatches.length - 1;
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
      searchStep(e.shiftKey ? 1 : -1);
    }
  }

  function segStyle(s: Segment): string {
    const parts: string[] = [];
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
  // masking. (Color words are applied upstream in the Go facade.) Search is
  // painted last so its active query wins visually.
  function displaySegs(line: Line): Segment[] {
    let segs = applyHighlights(segsFor(line), highlights);
    if (hideIPs) segs = maskIPs(segs);
    const query = searchQuery.trim().toLowerCase();
    if (store.searchOpen && query) {
      segs = applyHighlights(segs, [{ pattern: query, bg: SEARCH_STYLE.bg, fg: SEARCH_STYLE.fg }]);
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
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="pane"
    class:following={autoFollow}
    bind:this={viewport}
    onscroll={onScroll}
    onwheel={onViewportWheel}
    ontouchstart={onViewportTouchStart}
    ontouchmove={onViewportTouchMove}
    ontouchend={onViewportTouchEnd}
    ontouchcancel={onViewportTouchEnd}
    style="font-size:{fontSize}px"
  >
    <div class="content" bind:this={contentEl}>
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
  /* While following, application code owns tail anchoring. When detached,
     restore native anchoring so a reader's scrollback position stays stable. */
  .pane.following {
    overflow-anchor: none;
  }
  .content {
    min-width: 0;
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
