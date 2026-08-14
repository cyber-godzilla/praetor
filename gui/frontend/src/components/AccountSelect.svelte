<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let busy = $state("");
  let error = $state("");
  const canManageAccounts = $derived(
    store.credentialStore.available && store.credentialStore.canStore,
  );

  async function refreshAccounts() {
    const state = await api.listAccounts();
    store.accounts = state.accounts ?? [];
    store.credentialStore = state.credentialStore;
  }

  async function connect(username: string) {
    if (busy) return;
    busy = username;
    error = "";
    store.disconnectNotice = "";
    try {
      await api.connectStored(username);
      // The shared connected event can arrive before the HTTP operation
      // completes. Keep the game screen if that authoritative event won the
      // race; otherwise wait on the normal connecting screen.
      if (store.connState !== "connected") store.screen = "connecting";
    } catch (e: any) {
      error = e?.message ?? String(e);
      busy = "";
      try {
        await refreshAccounts();
      } catch {
        // Preserve the connection error; account health is best-effort here.
      }
    }
  }

  async function remove(username: string, ev: MouseEvent) {
    ev.stopPropagation();
    if (
      api.inWeb() &&
      !window.confirm(`Remove the stored TEC credentials for ${username} for every client?`)
    ) {
      return;
    }
    try {
      await api.removeAccount(username);
      await refreshAccounts();
      if (store.accounts.length === 0) store.screen = "login";
    } catch (cause: any) {
      error = cause?.message ?? String(cause);
    }
  }

</script>

<div class="wrap">
  <div class="card">
    <div class="brand">PRAETOR</div>
    <p class="sub">The Eternal City</p>

    {#if store.disconnectNotice}
      <div class="notice">
        <span>Connection lost: {store.disconnectNotice}.</span>
        <button class="notice-x" onclick={() => (store.disconnectNotice = "")}
          aria-label="Dismiss" type="button">✕</button>
      </div>
    {/if}

    <div class="section-label">Choose an account</div>
    <div class="accounts">
      {#each store.accounts as acct (acct)}
        <div class="acct" class:busy={!!busy}>
          <button class="acct-main" onclick={() => connect(acct)} disabled={!!busy} type="button">
            <span class="name">{acct}</span>
            {#if busy === acct}
              <span class="state">connecting…</span>
            {:else}
              <span class="go">›</span>
            {/if}
          </button>
          {#if busy !== acct}
            <button class="del" onclick={(e) => remove(acct, e)} disabled={!!busy || !canManageAccounts}
              title={canManageAccounts ? "Remove account" : "Credential storage unavailable"}
              aria-label="Remove account" type="button">✕</button>
          {/if}
        </div>
      {/each}
    </div>

    {#if error}<div class="err">{error}</div>{/if}
    {#if !canManageAccounts && store.credentialStore.message}
      <div class="storage-note">{store.credentialStore.message}</div>
    {/if}

    <button class="add" onclick={() => (store.screen = "login")} disabled={!!busy} type="button">
      + Add another account
    </button>
    {#if api.inWeb()}
      <button class="signout" onclick={() => void api.quit()} disabled={!!busy} type="button">
        Sign out of web UI
      </button>
    {/if}
    <div class="ver">{store.version}</div>
  </div>
</div>

<style>
  .wrap {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
    padding: 16px;
  }
  .card {
    width: 400px;
    max-width: 100%;
    background: var(--bg-panel);
    border: 1px solid var(--accent);
    padding: 24px 24px 16px;
  }
  .brand {
    text-align: center;
    font-size: 26px;
    font-weight: 700;
    letter-spacing: 8px;
    color: var(--accent);
  }
  .sub {
    margin: 4px 0 26px;
    text-align: center;
    font-size: 13px;
    color: var(--fg-dim);
    letter-spacing: 1px;
  }
  .notice {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
    padding: 8px 10px;
    border: 1px solid var(--red);
    border-radius: 8px;
    background: rgba(204, 68, 68, 0.12);
    color: var(--fg);
    font-size: 12px;
  }
  .notice span {
    flex: 1;
  }
  .notice-x {
    border: none;
    background: transparent;
    color: var(--fg-dim);
    padding: 0 4px;
  }
  .notice-x:hover {
    color: var(--red);
    background: transparent;
  }
  .section-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1.5px;
    color: var(--fg-dim);
    margin-bottom: 10px;
  }
  .accounts {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .storage-note {
    margin-top: 12px;
    color: var(--fg-dim);
    font-size: 12px;
    line-height: 1.4;
  }
  .acct {
    display: flex;
    align-items: stretch;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
    transition: border-color 0.12s, background 0.12s;
  }
  .acct:hover {
    border-color: var(--accent);
    background: var(--bg-hover);
  }
  .acct-main {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    background: transparent;
    border: none;
    border-radius: 0;
    text-align: left;
  }
  .acct-main:hover {
    background: transparent;
  }
  .acct-main:active:not(:disabled) {
    transform: translateY(1px);
  }
  .name {
    flex: 1;
    font-family: var(--mono);
    font-size: 16px;
    font-weight: 600;
  }
  .state {
    color: var(--fg-dim);
    font-size: 13px;
  }
  .go {
    color: var(--fg-dim);
    font-size: 20px;
  }
  .del {
    color: var(--fg-dim);
    padding: 0 14px;
    border: none;
    border-left: 1px solid var(--border);
    border-radius: 0;
    background: transparent;
    font-size: 13px;
  }
  .del:hover {
    color: var(--red);
    background: rgba(204, 68, 68, 0.12);
  }
  .add {
    margin-top: 18px;
    width: 100%;
    background: transparent;
    border: 1px dashed var(--border-bright);
    padding: 11px;
    color: var(--fg-dim);
  }
  .add:hover:not(:disabled) {
    color: var(--fg);
    border-color: var(--accent);
    background: transparent;
  }
  .err {
    color: var(--red);
    margin-top: 14px;
    font-size: 13px;
    text-align: center;
  }
  .ver {
    margin-top: 18px;
    text-align: center;
    font-size: 11px;
    color: var(--fg-dim);
  }
  .signout {
    width: 100%;
    margin-top: 8px;
    border: none;
    background: none;
    color: var(--fg-dim);
  }
</style>
