<script lang="ts">
  import type { Tab, Line } from "../lib/store.svelte";
  import { store } from "../lib/store.svelte";
  import type { Segment } from "../lib/types";
  import { applyHighlights, compileHighlights, maskIPs } from "../lib/highlight";

  let { tab }: { tab: Tab } = $props();

  const highlights = $derived(compileHighlights(store.config?.Highlights));
  const hideIPs = $derived(!!store.config?.UI?.HideIPs);

  let viewport: HTMLDivElement;
  let atBottom = $state(true);

  function onScroll() {
    if (!viewport) return;
    const gap = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
    atBottom = gap < 40;
  }

  // Auto-scroll to bottom when new lines arrive and the user is already there.
  $effect(() => {
    // Touch length so the effect re-runs on append.
    void tab.lines.length;
    if (atBottom && viewport) {
      queueMicrotask(() => {
        viewport.scrollTop = viewport.scrollHeight;
      });
    }
  });

  function segStyle(s: Segment): string {
    const parts: string[] = [];
    if (s.color) parts.push(`color:${s.color}`);
    if (s.bg) parts.push(`background:${s.bg};border-radius:3px;padding:0 2px`);
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
      return line.suppressed.revealed ? line.suppressed.original : line.suppressed.placeholder;
    }
    return line.segments;
  }

  function isBlank(line: Line): boolean {
    const s = segsFor(line);
    return s.length === 0 || (s.length === 1 && s[0].text === "" && !s[0].isHR);
  }
</script>

<div class="pane" bind:this={viewport} onscroll={onScroll}>
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
        tabindex={line.suppressed ? 0 : undefined}
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
    /* Virtualization: the browser skips layout/paint for off-screen lines,
       keeping long scrollback (thousands of lines) smooth without a manual
       windowing implementation. */
    content-visibility: auto;
    contain-intrinsic-size: auto 1.4em;
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
