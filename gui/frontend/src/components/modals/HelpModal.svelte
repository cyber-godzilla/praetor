<script lang="ts">
  import Modal from "../Modal.svelte";

  const keys: [string, string][] = [
    ["Tab / Shift+Tab", "Next / previous tab"],
    ["Alt+1…9, Alt+0", "Jump to tab N"],
    ["Alt+S", "Toggle sidebar"],
    ["Alt+M", "Quick-cycle modes"],
    ["Esc", "Open menu"],
    ["↑ / ↓", "Command history"],
    ["Enter (empty)", "Send a blank line"],
  ];

  const cmds: [string, string][] = [
    ["/help", "Show this help"],
    ["/list", "List available Lua modes"],
    ["/mode <name> [args], /sm", "Switch mode (validated)"],
    ["/toggle <label>", "Toggle a mode state value"],
    ["/set <label> <value>", "Set a mode state value"],
    ["/calc, /rb", "Rank-bonus calculator"],
    ["/wiki [name]", "Open a wiki bookmark"],
    ["/maps [name]", "Open a map bookmark"],
  ];
</script>

<Modal title="Help" wide back>
  <div class="cols">
    <div class="col">
      <div class="h dim">Key bindings</div>
      <table>
        <tbody>
          {#each keys as [k, d] (k)}
            <tr><td class="k">{k}</td><td>{d}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="col">
      <div class="h dim">Commands</div>
      <table>
        <tbody>
          {#each cmds as [k, d] (k)}
            <tr><td class="k">{k}</td><td>{d}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</Modal>

<style>
  .cols {
    display: flex;
    gap: 24px;
  }
  .col {
    flex: 1;
  }
  .h {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 8px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  td {
    padding: 4px 6px;
    vertical-align: top;
  }
  .k {
    font-family: var(--mono);
    color: var(--accent);
    white-space: nowrap;
    padding-right: 12px;
  }
</style>
