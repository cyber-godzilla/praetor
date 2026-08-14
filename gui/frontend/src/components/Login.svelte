<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let username = $state("");
  let password = $state("");
  let storeCredentials = $state(
    store.credentialStore.available && store.credentialStore.canStore,
  );
  let busy = $state(false);
  let error = $state("");
  const canRemember = $derived(
    store.credentialStore.available && store.credentialStore.canStore,
  );

  $effect(() => {
    if (!canRemember) storeCredentials = false;
  });

  async function submit(e: Event) {
    e.preventDefault();
    if (!username || !password) return;
    busy = true;
    error = "";
    store.disconnectNotice = "";
    try {
      const result = await api.connectNew(username, password, storeCredentials);
      password = "";
      store.loginUser = username;
      if (result.accountState) {
        store.accounts = result.accountState.accounts ?? [];
        store.credentialStore = result.accountState.credentialStore;
      }
      if (result.credentialsSaved && !store.accounts.includes(username)) {
        store.accounts = [...store.accounts, username].sort((a, b) => a.localeCompare(b));
      }
      if (result.warning) {
        store.addToast("Account not remembered", result.warning);
      }
      // The server starts the game loop before this HTTP request returns, so
      // its WebSocket "connected" event may already have advanced the UI to
      // the game screen. Never overwrite that newer authoritative state with
      // the transitional screen.
      if (store.connState !== "connected") store.screen = "connecting";
    } catch (err: any) {
      error = err?.message ?? String(err);
      busy = false;
    }
  }
</script>

<div class="wrap">
  <form class="card" onsubmit={submit}>
    <h1>PRAETOR</h1>
    <p class="dim sub">Sign in to The Eternal City</p>

    {#if store.disconnectNotice}
      <div class="notice">
        <span>Connection lost: {store.disconnectNotice}.</span>
        <button class="notice-x" onclick={() => (store.disconnectNotice = "")}
          aria-label="Dismiss" type="button">✕</button>
      </div>
    {/if}

    <label>Username
      <!-- svelte-ignore a11y_autofocus -->
      <input type="text" bind:value={username} autocomplete="username" autofocus />
    </label>
    <label>Password
      <input type="password" bind:value={password} autocomplete="current-password" />
    </label>

    <label class="check" class:disabled={!canRemember}>
      <input type="checkbox" bind:checked={storeCredentials} disabled={!canRemember} />
      Remember this account
    </label>

    {#if !canRemember && store.credentialStore.message}
      <div class="storage-note">{store.credentialStore.message}</div>
    {/if}

    {#if error}<div class="err">{error}</div>{/if}

    <button class="primary submit" type="submit" disabled={busy || !username || !password}>
      {busy ? "Connecting…" : "Connect"}
    </button>

    {#if store.accounts.length > 0}
      <button type="button" class="back" onclick={() => (store.screen = "account")} disabled={busy}>
        ← Back to accounts
      </button>
    {/if}
    {#if api.inWeb()}
      <button type="button" class="back" onclick={() => void api.quit()} disabled={busy}>
        Sign out of web UI
      </button>
    {/if}
  </form>
</div>

<style>
  .wrap {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
  }
  .card {
    width: 340px;
    max-width: 100%;
    background: var(--bg-panel);
    border: 1px solid var(--accent);
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  h1 {
    margin: 0;
    text-align: center;
    font-size: 26px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 8px;
  }
  .sub {
    margin: 0 0 8px;
    text-align: center;
    font-size: 13px;
  }
  .notice {
    display: flex;
    align-items: center;
    gap: 8px;
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
  label {
    display: flex;
    flex-direction: column;
    gap: 5px;
    font-size: 12px;
    color: var(--fg-dim);
  }
  label input[type="text"],
  label input[type="password"] {
    font-size: 14px;
  }
  .check {
    flex-direction: row;
    align-items: center;
    gap: 8px;
    color: var(--fg);
    font-size: 13px;
    cursor: pointer;
  }
  .check input {
    width: auto;
  }
  .check.disabled {
    cursor: default;
    color: var(--fg-dim);
  }
  .storage-note {
    margin-top: -6px;
    padding-left: 23px;
    color: var(--fg-dim);
    font-size: 11px;
    line-height: 1.35;
  }
  .submit {
    margin-top: 6px;
    padding: 10px;
    font-size: 15px;
  }
  .back {
    background: none;
    border: none;
    color: var(--fg-dim);
  }
  .back:hover {
    color: var(--fg);
    background: none;
  }
  .err {
    color: var(--red);
    font-size: 13px;
  }
</style>
