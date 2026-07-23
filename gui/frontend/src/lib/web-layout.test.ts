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

  it("corrects outer-page drift after the mobile software keyboard settles", () => {
    const input = source("../components/InputLine.svelte");
    const output = source("../components/OutputPane.svelte");

    expect(input).toContain("MOBILE_KEYBOARD_FALLBACK_MS");
    expect(input).toContain("MOBILE_KEYBOARD_OBSERVE_MS");
    expect(input).toContain('addEventListener("resize", scheduleKeyboardViewportCorrection)');
    expect(input).toContain('addEventListener("scroll", scheduleKeyboardViewportCorrection)');
    expect(input).toContain("Svelte has committed that shorter layout");
    expect(input).toContain("void tick().then(() =>");
    expect(input).toContain("outerPageHasVerticalDrift");
    expect(input).toContain("document.activeElement !== inputEl");
    expect(input).toContain("window.scrollTo(0, 0)");
    expect(input).not.toContain("viewport.scrollTop");
    expect(output).toContain("viewport.scrollTop");
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

  it("wires the browser-mobile preferences together at the bottom", () => {
    const settings = source("../components/modals/SettingsModal.svelte");
    const dock = source("../components/MobileDock.svelte");
    const input = source("../components/InputLine.svelte");
    const tabBar = source("../components/TabBar.svelte");
    const statusBar = source("../components/StatusBar.svelte");

    const toolbar = settings.indexOf("Show Actions / Modes / Menu row on mobile");
    const tabSelector = settings.indexOf("Show tab selector on mobile");
    const navigation = settings.indexOf("Hide map and compass while command input is active");
    const lowercase = settings.indexOf("Lowercase the first command letter on mobile");
    const mobileFont = settings.indexOf("Mobile output text size");
    const logPath = settings.lastIndexOf("bind:value={logPath}");
    expect(toolbar).toBeGreaterThan(-1);
    expect(toolbar).toBeGreaterThan(logPath);
    expect(tabSelector).toBeGreaterThan(toolbar);
    expect(navigation).toBeGreaterThan(tabSelector);
    expect(lowercase).toBeGreaterThan(navigation);
    expect(mobileFont).toBeGreaterThan(lowercase);

    expect(settings).toContain("{#if api.inWeb()}");
    expect(dock).toContain("MobileShowToolbar");
    expect(tabBar).toContain("MobileShowTabBar");
    expect(tabBar).toContain("hide-on-mobile");
    expect(statusBar).toContain("MobileShowTabBar");
    expect(statusBar).toContain("mobile-menu-btn");
    expect(statusBar).toContain("conn-label");
    expect(statusBar).toContain(".conn.ok .conn-label");
    expect(statusBar).not.toContain("min-height: 44px");
    expect(dock).toContain("MobileHideNavigationOnInput");
    expect(dock).toContain("store.mobileCommandInputActive");
    expect(input).toContain("lowercaseFirstCommandLetter");
    expect(input).toContain('autocapitalize={api.inWeb()');
    expect(input).toContain("onfocus={onFocus}");

    const output = source("../components/OutputPane.svelte");
    expect(settings).toContain("MOBILE_FONT_SIZES = [6,");
    expect(output).toContain("MobileOutputFontSize");
    expect(output).toContain("MOBILE_LAYOUT_QUERY");
    expect(output).toContain("outputFontSizeForLayout");
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
