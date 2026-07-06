<script lang="ts">
  import Modal from "../Modal.svelte";
  import * as api from "../../lib/bridge";
  import type { WikiSection } from "../../lib/types";

  let { title, kind }: { title: string; kind: "wiki" | "maps" } = $props();

  let sections = $state<WikiSection[]>([]);
  let query = $state("");

  $effect(() => {
    (async () => {
      sections = kind === "wiki" ? await api.getWikiSections() : await api.getMapSections();
    })();
  });

  const filtered = $derived.by(() => {
    const q = query.trim().toLowerCase();
    if (!q) return sections;
    return sections
      .map((s) => ({
        ...s,
        Bookmarks: (s.Bookmarks ?? []).filter((b) => b.Key.toLowerCase().includes(q)),
      }))
      .filter((s) => s.Bookmarks.length > 0);
  });

  function open(slug: string) {
    api.openWikiSlug(slug);
  }
</script>

<Modal {title} wide back>
  <input class="search" type="text" bind:value={query} placeholder="Filter…" />
  <div class="sections">
    {#each filtered as sec (sec.Name)}
      <div class="section">
        <div class="sname dim">{sec.Name}</div>
        <div class="marks">
          {#each sec.Bookmarks ?? [] as bm (bm.Slug)}
            <button class="mark" onclick={() => open(bm.Slug)}>
              <span class="arrow">›</span>{bm.Key}
            </button>
          {/each}
        </div>
      </div>
    {/each}
    {#if filtered.length === 0}<div class="dim empty">No matches.</div>{/if}
  </div>
</Modal>

<style>
  .search {
    width: 100%;
    margin-bottom: 14px;
  }
  .sections {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .sname {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 6px;
  }
  /* Columnar list: bookmarks flow top-to-bottom into responsive columns. */
  .marks {
    column-width: 200px;
    column-gap: 14px;
  }
  .mark {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    break-inside: avoid;
    text-align: left;
    background: none;
    border: none;
    border-radius: 4px;
    padding: 5px 8px;
    font-size: 13px;
    color: var(--fg);
  }
  .mark:hover {
    background: var(--bg-elevated);
    color: var(--accent);
  }
  .arrow {
    color: var(--fg-dim);
  }
  .mark:hover .arrow {
    color: var(--accent);
  }
  .empty {
    padding: 20px;
    text-align: center;
  }
</style>
