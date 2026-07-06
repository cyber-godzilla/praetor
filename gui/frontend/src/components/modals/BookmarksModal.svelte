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

<Modal {title} wide>
  <input class="search" type="text" bind:value={query} placeholder="Filter…" />
  <div class="sections">
    {#each filtered as sec (sec.Name)}
      <div class="section">
        <div class="sname dim">{sec.Name}</div>
        <div class="marks">
          {#each sec.Bookmarks ?? [] as bm (bm.Slug)}
            <button class="mark" onclick={() => open(bm.Slug)}>{bm.Key}</button>
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
  .marks {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .mark {
    font-size: 12px;
    padding: 5px 10px;
  }
  .empty {
    padding: 20px;
    text-align: center;
  }
</style>
