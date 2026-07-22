import { describe, expect, it } from "vitest";
// The production tsconfig intentionally omits Node types; Vitest supplies the
// built-in at test time for these source-level responsive contracts.
// @ts-expect-error node:fs types are test-runner-only
import { readFileSync } from "node:fs";

const source = (relative: string) =>
  readFileSync(new URL(relative, import.meta.url), "utf8");

describe("web responsive layout contract", () => {
  it("keeps the mobile map/navigation dock after output and command input", () => {
    const game = source("../components/GameView.svelte");
    expect(game.indexOf("<OutputPane")).toBeGreaterThan(-1);
    expect(game.indexOf("<InputLine")).toBeGreaterThan(game.indexOf("<OutputPane"));
    expect(game.indexOf("<MobileDock")).toBeGreaterThan(game.indexOf("<InputLine"));
    expect(game).toContain("@media (max-width: 899px)");
  });

  it("retains touch, visual-viewport, dynamic-height, and safe-area behavior", () => {
    const app = source("../App.svelte");
    const styles = source("../styles.css");
    const input = source("../components/InputLine.svelte");
    const navigation = source("../components/MapNavigation.svelte");
    const dock = source("../components/MobileDock.svelte");

    expect(app).toContain("window.visualViewport");
    expect(styles).toContain("100dvh");
    expect(styles).toMatch(/font-size:\s*16px\s*!important/);
    expect(input).toContain('class="send"');
    expect(input).toContain("pointer: coarse");
    expect(navigation).toMatch(/min-(?:width|height):\s*44px/);
    expect(dock).toContain("env(safe-area-inset-bottom)");
  });

  it("re-enables the command input before restoring desktop focus after submit", () => {
    const input = source("../components/InputLine.svelte");
    const finallyBlock = input.indexOf("} finally {");
    const enabled = input.indexOf("submitting = false;", finallyBlock);
    const rendered = input.indexOf("await tick();", enabled);
    const focused = input.indexOf("inputEl?.focus();", rendered);

    expect(finallyBlock).toBeGreaterThan(-1);
    expect(enabled).toBeGreaterThan(finallyBlock);
    expect(rendered).toBeGreaterThan(enabled);
    expect(focused).toBeGreaterThan(rendered);
    expect(input.slice(rendered, focused)).toContain("stickyFocusEnabled()");
  });

  it("does not let a completed login request overwrite an earlier connected event", () => {
    for (const component of ["../components/Login.svelte", "../components/AccountSelect.svelte"]) {
      const login = source(component);
      const awaitConnect = login.indexOf("await api.connect");
      const connectedGuard = login.indexOf('store.connState !== "connected"', awaitConnect);
      const connectingScreen = login.indexOf('store.screen = "connecting"', connectedGuard);

      expect(awaitConnect, component).toBeGreaterThan(-1);
      expect(connectedGuard, component).toBeGreaterThan(awaitConnect);
      expect(connectingScreen, component).toBeGreaterThan(connectedGuard);
    }
  });
});
