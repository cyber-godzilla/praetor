<script lang="ts">
  import type { Tab, Line } from "../lib/store.svelte";
  import { store } from "../lib/store.svelte";
  import type { Segment } from "../lib/types";
  import { applyHighlights, compileHighlights, maskIPs } from "../lib/highlight";

  let { tab }: { tab: Tab } = $props();

  const highlights = $derived(compileHighlights(store.config?.Highlights));
  const hideIPs = $derived(!!store.config?.UI?.HideIPs);
  const fontSize = $derived(store.config?.UI?.OutputFontSize || 14);

  let viewport: HTMLDivElement;
  // Follow the tail until the user scrolls up. Crucially we do NOT disengage on
  // "gap from bottom" — under a burst, appended lines grow scrollHeight before
  // the auto-scroll catches up, which would spuriously flip following off and
  // make the view fall progressively behind. Only an actual upward scroll (or
  // the user returning to the bottom) changes the follow state.
  let autoFollow = true;
  let lastTop = 0;

  function onScroll() {
    if (!viewport) return;
    const top = viewport.scrollTop;
    const gap = viewport.scrollHeight - top - viewport.clientHeight;
    if (gap <= 4) {
      autoFollow = true; // at the bottom → follow
    } else if (top < lastTop - 2) {
      autoFollow = false; // user scrolled up → stop following
    }
    lastTop = top;
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

  $effect(() => {
    // Touch length so the effect re-runs on append.
    void tab.lines.length;
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

  // PgUp/PgDn scroll the pane by ~85% of a page (mirrors the TUI). Home/End are
  // left to the input line for cursor movement.
  function onWindowKey(e: KeyboardEvent) {
    if (store.openModal || !viewport) return;
    if (e.key === "PageUp" || e.key === "PageDown") {
      e.preventDefault();
      const dir = e.key === "PageUp" ? -1 : 1;
      viewport.scrollBy({ top: viewport.clientHeight * 0.85 * dir });
      onScroll();
    }
  }

  function isBlank(line: Line): boolean {
    const s = segsFor(line);
    return s.length === 0 || (s.length === 1 && s[0].text === "" && !s[0].isHR);
  }
</script>

<svelte:window onkeydown={onWindowKey} />

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

<style>
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
