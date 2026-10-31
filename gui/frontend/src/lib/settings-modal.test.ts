import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

function source(path: string): string {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
}

describe("settings modal shell", () => {
  it("keeps settings routes at one stable desktop size", () => {
    const modal = source("../components/Modal.svelte");
    expect(modal).toContain("class:stable-settings={stableSettings}");
    expect(modal).toMatch(/\.modal\.stable-settings\s*\{[^}]*width:\s*1040px;[^}]*height:\s*min\(740px, 86vh\)/s);
    expect(modal).toMatch(/\.modal\.stable-settings \.modal-content\s*\{\s*flex:\s*1;/);
  });
});
