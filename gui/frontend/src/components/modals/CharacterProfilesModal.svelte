<script lang="ts">
  import Modal from "../Modal.svelte";
  import { store } from "../../lib/store.svelte";
  import * as api from "../../lib/bridge";
  import type { CharacterProfileIdentity } from "../../lib/types";

  function newID(): string {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
      return crypto.randomUUID();
    }
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    const hex = [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }

  const defaultID = store.config?.CharacterProfiles.DefaultID ?? "";
  let identities = $state<CharacterProfileIdentity[]>(
    (store.config?.CharacterProfiles.Profiles ?? []).map((profile) => ({
      ID: profile.ID,
      Name: profile.Name,
      Account: profile.Account ?? "",
      CharacterNumber: profile.CharacterNumber ?? 0,
    })),
  );
  let autoSwitch = $state(store.config?.CharacterProfiles.AutoSwitch ?? true);
  let error = $state("");
  let saving = $state(false);

  function preferredAccount(): string {
    return store.loginUser || store.accounts[0] || "";
  }

  function nextNumber(account: string): number {
    const used = new Set(
      identities
        .filter((profile) => profile.Account.toLocaleLowerCase() === account.toLocaleLowerCase())
        .map((profile) => profile.CharacterNumber),
    );
    let number = 1;
    while (used.has(number)) number += 1;
    return number;
  }

  function addProfile() {
    const account = preferredAccount();
    identities.push({
      ID: newID(),
      Name: "New character",
      Account: account,
      CharacterNumber: nextNumber(account),
    });
  }

  $effect(() => {
    if (store.newCharacterProfileRequest > 0) {
      store.newCharacterProfileRequest = 0;
      addProfile();
    }
  });

  function removeProfile(id: string) {
    const profile = identities.find((candidate) => candidate.ID === id);
    if (!profile || !confirm(`Delete the character profile “${profile.Name}”?\n\nIts layout, overrides, and character Lua data will be removed.`)) return;
    identities = identities.filter((profile) => profile.ID !== id);
  }

  function validate(): string {
    const seen = new Set<string>();
    const names = new Set<string>();
    for (const profile of identities) {
      if (profile.ID === defaultID) continue;
      profile.Name = profile.Name.trim();
      profile.Account = profile.Account.trim();
      if (!profile.Name) return "Every character profile needs a name.";
      const nameKey = profile.Name.toLocaleLowerCase();
      if (nameKey === "default" || nameKey === "global") return `${profile.Name} is reserved for the global Default profile.`;
      if (names.has(nameKey)) return `Character profile names must be unique; ${profile.Name} is used more than once.`;
      names.add(nameKey);
      if (!profile.Account) return `Choose an account for ${profile.Name}.`;
      if (!Number.isInteger(profile.CharacterNumber) || profile.CharacterNumber <= 0) {
        return `${profile.Name} needs a positive character number.`;
      }
      const key = `${profile.Account.toLocaleLowerCase()}\u0000${profile.CharacterNumber}`;
      if (seen.has(key)) return `${profile.Account} character ${profile.CharacterNumber} is assigned more than once.`;
      seen.add(key);
    }
    return "";
  }

  async function save() {
    if (saving || !store.config) return;
    error = validate();
    if (error) return;
    saving = true;
    try {
      await api.setCharacterProfiles(identities.map((profile) => ({ ...profile })));
      await api.setCharacterProfileAutoSwitch(autoSwitch);
      const saved = await api.getConfig();
      store.installConfig(saved);
      store.addToast("Character profiles", "Saved");
      store.openModal = null;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      saving = false;
    }
  }
</script>

<Modal title="Character Profiles" wide back onsave={save}>
  <p class="hint">
    Profiles keep variables, filtered views, and action sets separate for each character.
    Praetor can select a profile after you send <code>play N</code>, or <code>play</code> followed by <code>N</code>.
    Sending <code>sleep</code> returns to Default.
  </p>
  <label class="auto-switch">
    <input type="checkbox" bind:checked={autoSwitch} />
    <span><strong>Automatically follow play and sleep commands</strong>
      <small>When disabled, those commands leave the active profile unchanged.</small>
    </span>
  </label>
  <datalist id="character-profile-accounts">
    {#each store.accounts as account (account)}<option value={account}></option>{/each}
    {#if store.loginUser && !store.accounts.includes(store.loginUser)}<option value={store.loginUser}></option>{/if}
  </datalist>
  <div class="profiles">
    {#each identities as profile (profile.ID)}
      <div class="profile-row" class:active={profile.ID === store.activeCharacterProfileID}>
        {#if profile.ID === defaultID}
          <div class="default-name">Default <span class="tag">Global template</span></div>
          <div class="default-note">Used before a character is selected. New profiles inherit these settings.</div>
        {:else}
          <label>Name
            <input aria-label="Profile name" type="text" bind:value={profile.Name} />
          </label>
          <label>Account
            <input aria-label="Account" type="text" list="character-profile-accounts" bind:value={profile.Account} />
          </label>
          <label class="number">Character number
            <input aria-label="Character number" type="number" min="1" step="1" bind:value={profile.CharacterNumber} />
          </label>
          <button class="danger delete" aria-label="Delete profile" title="Delete profile"
            onclick={() => removeProfile(profile.ID)}>✕</button>
        {/if}
        {#if profile.ID === store.activeCharacterProfileID}<span class="active-tag">Active</span>{/if}
      </div>
    {/each}
  </div>
  <button class="add" aria-label="Add character profile" onclick={addProfile}>+ Add character profile</button>
  {#if error}<div class="err" role="alert">{error}</div>{/if}
</Modal>

<style>
  .hint { margin: 0 0 14px; color: var(--fg-dim); font-size: 12px; line-height: 1.45; }
  code { color: var(--accent); }
  .profiles { display: flex; flex-direction: column; gap: 8px; }
  .auto-switch {
    display: flex;
    flex-direction: row;
    align-items: flex-start;
    gap: 8px;
    margin: 0 0 14px;
    color: var(--fg);
    font-size: 12px;
  }
  .auto-switch input { width: auto; margin-top: 2px; }
  .auto-switch span { display: flex; flex-direction: column; gap: 2px; }
  .auto-switch small { color: var(--fg-dim); }
  .profile-row {
    position: relative;
    display: grid;
    grid-template-columns: minmax(150px, 1.25fr) minmax(140px, 1fr) 130px auto;
    gap: 8px;
    align-items: end;
    padding: 10px;
    border: 1px solid var(--border);
    background: var(--bg-elevated);
  }
  .profile-row.active { border-color: var(--accent); }
  label { display: flex; flex-direction: column; gap: 4px; color: var(--fg-dim); font-size: 11px; }
  input { min-width: 0; width: 100%; }
  .default-name { font-weight: 700; align-self: center; }
  .default-note { grid-column: 2 / 5; color: var(--fg-dim); font-size: 12px; align-self: center; }
  .tag, .active-tag {
    margin-left: 7px;
    color: var(--fg-dim);
    font-size: 10px;
    font-weight: 400;
    text-transform: uppercase;
    letter-spacing: .6px;
  }
  .active-tag {
    position: absolute;
    top: 0;
    right: 10px;
    transform: translateY(-50%);
    margin-left: 0;
    padding: 0 5px;
    color: var(--accent);
    background: var(--bg-elevated);
    line-height: 1.2;
  }
  .delete { align-self: end; }
  .add { margin-top: 10px; }
  .err { margin-top: 10px; color: var(--red); font-size: 12px; }
  @media (max-width: 680px) {
    .profile-row { grid-template-columns: 1fr auto; }
    .profile-row label, .default-note { grid-column: 1 / -1; }
    .delete { grid-column: 2; grid-row: 1; }
  }
</style>
