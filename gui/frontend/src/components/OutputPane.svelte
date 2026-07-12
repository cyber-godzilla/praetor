<script lang="ts">
  import type { Tab, Line } from "../lib/store.svelte";
  import { store } from "../lib/store.svelte";
  import type { Segment } from "../lib/types";
  import { applyHighlights, compileHighlights, maskIPs } from "../lib/highlight";
  import { followBandPx, gapToBottom, withinBand } from "../lib/scroll";

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
  // Home freezes at the top even when the whole buffer is short enough that the
  // top sits inside the follow band. Setting scrollTop=0 fires a scroll event
  // that would otherwise re-engage following, so we swallow exactly that one
  // programmatic event.
  let ignoreScroll = false;

  function bandPx(): number {
    return followBandPx(fontSize);
  }

  // Recompute the follow state from the current scroll position.
  function onScroll() {
    if (!viewport) return;
    if (ignoreScroll) {
      ignoreScroll = false;
      return;
    }
    autoFollow = withinBand(gapToBottom(viewport), bandPx());
  }

  // Coalesce all appends within a frame into a single scroll-to-bottom.
  let scrollQueued = false;
  function followTail() {
    if (scrollQueued) return;
    scrollQueued = true;
    requestAnimationFrame(() => {
      scrollQueued = false;
      if (viewport && autoFollow) viewport.scrollTop = viewport.scrollHeight;
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
  }
  function toEnd() {
    if (!viewport) return;
    autoFollow = true;
    viewport.scrollTop = viewport.scrollHeight;
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
  });

  // Re-anchor to the bottom when the viewport geometry changes (window/sidebar
  // resize, font-size change). Without this, scrollHeight/clientHeight shift
  // while scrollTop stays put, leaving the "followed" view stuck at an offset.
  $effect(() => {
    if (!viewport) return;
    const ro = new ResizeObserver(() => {
      if (autoFollow) followTail();
    });
    ro.observe(viewport);
    return () => ro.disconnect();
  });

  // Font-size changes reflow line heights without resizing the viewport, so
  // re-follow explicitly when it changes.
  $effect(() => {
    void fontSize;
    if (autoFollow) followTail();
  });

  // Snap to the bottom and resume following when switching to another tab.
  $effect(() => {
    void tab.name;
    autoFollow = true;
    followTail();
  });

  function segStyle(s: Segment): string {
    const parts: string[] = [];
    if (s.color) parts.push(`color:${s.color}`);
    // Background only — no padding/radius, so a highlight tints the exact
    // character cells like the TUI and doesn't add horizontal space.
    if (s.bg) parts.push(`background:${s.bg}`);
    if (s.bold) parts.push("font-weight:700");
    if (s.italic) parts.push("font-style:italic");
    if (s.underline) parts.push("text-decoration:underline");
    return parts.join(";");
  }

  // Apply frontend render transforms in the TUI order: highlights, then IP
  // masking. (Color words are applied upstream in the Go facade.)
  function displaySegs(line: Line): Segment[] {
    let segs = applyHighlights(segsFor(line), highlights);
    if (hideIPs) segs = maskIPs(segs);
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

  <!-- Roman-style scroll controls pinned to the top and bottom of the
       scrollbar: single arrows page, barred arrows jump to top/bottom. -->
  <div class="scroll-ctl scroll-ctl-top">
    <button type="button" tabindex="-1" title="Top (Home)" aria-label="Scroll to top" onclick={toTop}>
      <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="3" y="3" width="10" height="1.7" /><path d="M8 6 L13 13 L3 13 Z" /></svg>
    </button>
    <button type="button" tabindex="-1" title="Page up" aria-label="Page up" onclick={() => pageBy(-1)}>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 4 L13 11 L3 11 Z" /></svg>
    </button>
  </div>
  <div class="scroll-ctl scroll-ctl-bottom">
    <button type="button" tabindex="-1" title="Page down" aria-label="Page down" onclick={() => pageBy(1)}>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 12 L3 5 L13 5 Z" /></svg>
    </button>
    <button type="button" tabindex="-1" title="Bottom (End)" aria-label="Scroll to bottom" onclick={toEnd}>
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M8 10 L3 3 L13 3 Z" /><rect x="3" y="12" width="10" height="1.7" /></svg>
    </button>
  </div>
</div>

<style>
  .output {
    flex: 1;
    min-height: 0;
    position: relative;
    display: flex;
  }
  .pane {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 8px 12px;
    font-family: var(--mono);
    font-size: 14px;
    line-height: 1.4;
    background: var(--bg);
    user-select: text;
  }
  /* Two clusters overlaying the top and bottom ends of the scrollbar. Faint at
     rest so they don't fight the text; each button brightens to the Skotos
     accent on hover. */
  .scroll-ctl {
    position: absolute;
    right: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 3px;
    z-index: 2;
    opacity: 0.45;
    transition: opacity 0.12s;
  }
  .scroll-ctl-top {
    top: 0;
  }
  .scroll-ctl-bottom {
    bottom: 0;
  }
  .output:hover .scroll-ctl {
    opacity: 0.85;
  }
  .scroll-ctl button {
    width: 20px;
    height: 20px;
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
  .scroll-ctl button:hover {
    color: var(--accent);
    border-color: var(--border-bright);
  }
  .scroll-ctl svg {
    width: 13px;
    height: 13px;
    fill: currentColor;
  }
  .line {
    white-space: pre-wrap;
    word-break: break-word;
    min-height: 1.4em;
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
