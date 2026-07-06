<script lang="ts">
  import { store } from "../lib/store.svelte";
  import * as api from "../lib/bridge";

  let username = $state("");
  let password = $state("");
  let storeCredentials = $state(true);
  let busy = $state(false);
  let error = $state("");

  async function submit(e: Event) {
    e.preventDefault();
    if (!username || !password) return;
    busy = true;
    error = "";
    try {
      await api.connectNew(username, password, storeCredentials);
      store.loginUser = username;
      if (storeCredentials && !store.accounts.includes(username)) {
        store.accounts = [...store.accounts, username];
      }
      store.screen = "connecting";
    } catch (err: any) {
      error = err?.message ?? String(err);
      busy = false;
    }
  }
</script>

<div class="wrap">
  <form class="card" onsubmit={submit}>
    <h1>Praetor</h1>
    <p class="dim sub">Sign in to The Eternal City</p>

    <label>Username
      <!-- svelte-ignore a11y_autofocus -->
      <input type="text" bind:value={username} autocomplete="username" autofocus />
    </label>
    <label>Password
      <input type="password" bind:value={password} autocomplete="current-password" />
    </label>

    <label class="check">
      <input type="checkbox" bind:checked={storeCredentials} />
      Remember this account
    </label>

    {#if error}<div class="err">{error}</div>{/if}

    <button class="primary submit" type="submit" disabled={busy || !username || !password}>
      {busy ? "Connecting…" : "Connect"}
    </button>

    {#if store.accounts.length > 0}
      <button type="button" class="back" onclick={() => (store.screen = "account")} disabled={busy}>
        ← Back to accounts
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
  }
  .card {
    width: 360px;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 32px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  h1 {
    margin: 0;
    text-align: center;
    font-size: 32px;
    color: var(--accent);
    letter-spacing: 2px;
  }
  .sub {
    margin: 0 0 8px;
    text-align: center;
    font-size: 13px;
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
