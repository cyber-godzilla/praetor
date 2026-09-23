<script lang="ts">
  import Modal from "../Modal.svelte";
  import VariablesTab from "../sidebar/VariablesTab.svelte";
</script>

<Modal title="Variables" back>
  <p class="hint">
    Define reusable values for typed commands and action-set buttons. Reference them as
    <code>{"${name}"}</code>.
  </p>
  <VariablesTab />
</Modal>

<style>
  .hint {
    margin: 0 0 12px;
    color: var(--fg-dim);
    font-size: 12px;
    line-height: 1.45;
  }
  code {
    color: var(--accent);
  }
</style>
