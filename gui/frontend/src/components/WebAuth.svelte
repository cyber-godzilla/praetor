<script lang="ts">
  import * as api from "../lib/bridge";

  let { onunlock }: { onunlock: () => void | Promise<void> } = $props();
  let password = $state("");
  let busy = $state(false);
  let error = $state("");

  async function submit(event: Event) {
    event.preventDefault();
    if (!password || busy) return;
    busy = true;
    error = "";
    try {
      await api.webLogin(password);
      password = "";
      await onunlock();
    } catch (cause: any) {
      error = cause?.message ?? String(cause);
    } finally {
      busy = false;
    }
  }
</script>

<div class="wrap">
  <form class="auth" onsubmit={submit}>
    <div class="brand">PRAETOR</div>
    <div class="sub dim">Shared web client</div>
    <label>
      Preshared password
      <!-- svelte-ignore a11y_autofocus -->
      <input
        type="password"
        bind:value={password}
        autocomplete="current-password"
        autofocus
        disabled={busy}
      />
    </label>
    {#if error}<div class="error">{error}</div>{/if}
    <button class="primary" type="submit" disabled={busy || !password}>
      {busy ? "Authenticating…" : "Unlock"}
    </button>
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
  .auth {
    width: 340px;
    max-width: 100%;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 24px;
    background: var(--bg-panel);
    border: 1px solid var(--accent);
  }
  .brand {
    text-align: center;
    color: var(--accent);
    font-size: 26px;
    font-weight: 700;
    letter-spacing: 8px;
  }
  .sub {
    margin-top: -10px;
    text-align: center;
    font-size: 12px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 5px;
    font-size: 12px;
    color: var(--fg-dim);
  }
  input {
    font-size: 14px;
  }
  button {
    padding: 9px;
  }
  .error {
    color: var(--red);
    font-size: 12px;
  }
</style>
