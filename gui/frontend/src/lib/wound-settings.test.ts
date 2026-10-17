import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";

const source = (relative: string) => readFileSync(new URL(relative, import.meta.url), "utf8");

describe("wound highlight settings", () => {
  it("exposes an opt-in toggle and four live-persisted family pickers", () => {
    const settings = source("../components/modals/SettingsModal.svelte");
    expect(settings).toContain("Highlight wounds");
    expect(settings).toContain("Cuts");
    expect(settings).toContain("Bruises / fractures");
    expect(settings).toContain("Punctures / wounds");
    expect(settings).toContain("Burns");
    expect(settings).toContain("api.setWoundHighlights");
    expect(settings).toContain("store.config.WoundHighlights");
  });

  it("keeps the native bridge and web transport in parity", () => {
    expect(source("./bridge.ts")).toContain('call<void>("SetWoundHighlights"');
    expect(source("./transport-web.ts")).toContain('SetWoundHighlights: "wound-highlights"');
  });
});
