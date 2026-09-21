<script lang="ts">
  import Modal from "../Modal.svelte";
  import * as api from "../../lib/bridge";

  const pages = [
    {
      label: "Praetor overview",
      description: "What Praetor is and the features available to you.",
      url: "https://eternal-city.wikidot.com/praetor",
    },
    {
      label: "Praetor guide",
      description: "A guided introduction to setup and everyday use.",
      url: "https://eternal-city.wikidot.com/praetor-guide",
    },
    {
      label: "Praetor scripts",
      description: "Install and use the community automation scripts.",
      url: "https://eternal-city.wikidot.com/praetor-scripts",
    },
  ];
</script>

<Modal title="Welcome to Praetor">
  <p class="intro">
    These wiki pages will help you get oriented. Nothing opens automatically;
    choose any page you would like to read.
  </p>
  <div class="pages">
    {#each pages as page (page.url)}
      <button class="page" onclick={() => api.openURL(page.url)} aria-label={page.label}>
        <span class="label">{page.label}</span>
        <span class="description">{page.description}</span>
        <span class="url">{page.url}</span>
      </button>
    {/each}
  </div>
</Modal>

<style>
  .intro {
    margin: 0 0 14px;
    color: var(--fg-dim);
    line-height: 1.45;
  }
  .pages {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .page {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 3px;
    padding: 10px 12px;
    text-align: left;
  }
  .label {
    color: var(--accent);
    font-weight: 700;
  }
  .description {
    color: var(--fg);
    font-size: 12px;
  }
  .url {
    color: var(--fg-dim);
    font-family: var(--mono);
    font-size: 10px;
  }
</style>
