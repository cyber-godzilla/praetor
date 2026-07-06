<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let busy = $state("");
  let error = $state("");

  async function connect(username: string) {
    busy = username;
    error = "";
    try {
      await api.connectStored(username);
      store.screen = "connecting";
    } catch (e: any) {
      error = e?.message ?? String(e);
      busy = "";
    }
  }

  async function remove(username: string, ev: MouseEvent) {
    ev.stopPropagation();
    await api.removeAccount(username);
    store.accounts = store.accounts.filter((a) => a !== username);
    if (store.accounts.length === 0) store.screen = "login";
  }
</script>

<div class="wrap">
  <div class="card">
    <h1>Praetor</h1>
    <p class="dim sub">The Eternal City — select an account</p>

    <div class="accounts">
      {#each store.accounts as acct (acct)}
        <div class="acct" class:busy={!!busy}>
          <button class="connect" onclick={() => connect(acct)} disabled={!!busy}>
            <span class="name">{acct}</span>
          </button>
          {#if busy === acct}
            <span class="dim state">connecting…</span>
          {:else}
            <button class="del" onclick={(e) => remove(acct, e)} disabled={!!busy}
              title="Remove account" aria-label="Remove account">✕</button>
          {/if}
        </div>
      {/each}
    </div>

    {#if error}<div class="err">{error}</div>{/if}

    <button class="add" onclick={() => (store.screen = "login")} disabled={!!busy}>
      + Add another account
    </button>
    <div class="ver dim">v{store.version}</div>
  </div>
</div>

<style>
  .wrap {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .card {
    width: 380px;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 32px;
    text-align: center;
  }
  h1 {
    margin: 0;
    font-size: 32px;
    color: var(--accent);
    letter-spacing: 2px;
  }
  .sub {
    margin: 6px 0 24px;
    font-size: 13px;
  }
  .accounts {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .acct {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .acct .connect {
    flex: 1;
    text-align: left;
    padding: 12px 16px;
    font-size: 15px;
  }
  .name {
    font-weight: 600;
  }
  .state {
    font-size: 13px;
    padding-right: 8px;
  }
  .del {
    color: var(--fg-dim);
    padding: 10px 12px;
  }
  .del:hover {
    color: var(--red);
    border-color: var(--red);
  }
  .add {
    margin-top: 20px;
    width: 100%;
  }
  .err {
    color: var(--red);
    margin-top: 12px;
    font-size: 13px;
  }
  .ver {
    margin-top: 18px;
    font-size: 11px;
  }
</style>
