import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { COMMANDS } from "./commands";

// Slash commands are dispatched in two places and documented in a third. Those
// drifted once already — /help sat four features out of date without anyone
// noticing. Scanning the dispatch sites is unconventional for a frontend test,
// and it is the only thing that makes the catalog self-maintaining: adding a
// command without a catalog entry fails here, naming the command.

const read = (rel: string) => readFileSync(new URL(rel, import.meta.url), "utf8");

function known(): Set<string> {
  const s = new Set<string>();
  for (const c of COMMANDS) {
    s.add(c.name);
    for (const a of c.aliases ?? []) s.add(a);
  }
  return s;
}

function matchAll(src: string, re: RegExp): string[] {
  return [...src.matchAll(re)].map((m) => m[1].toLowerCase());
}

function scanInputLine(): Set<string> {
  const src = read("../components/InputLine.svelte");
  // if (lower === "/play") { …   and   lower.startsWith("/notes ")
  return new Set([
    ...matchAll(src, /lower === "(\/[a-z]+)"/g),
    ...matchAll(src, /lower\.startsWith\("(\/[a-z]+)[ "]/g),
  ]);
}

function scanClientGo(): Set<string> {
  const src = read("../../../../internal/client/client.go");
  // Scan only the dispatch function, so an unrelated "/foo" string elsewhere
  // in the file cannot be mistaken for a command.
  const start = src.indexOf("func (c *Client) handleLocalCommand");
  expect(start, "handleLocalCommand not found — client.go was restructured").toBeGreaterThan(-1);
  // Bound the slice at the next top-level func, or the scan would run to EOF
  // and pick up slash-looking strings from unrelated helpers below.
  const rest = src.slice(start);
  const end = rest.indexOf("\nfunc ", 1);
  const body = end === -1 ? rest : rest.slice(0, end);

  // Cases look like:  case "/mode", "/sm":
  return new Set(matchAll(body, /"(\/[a-z-]+)"/g));
}

describe("catalog covers every dispatched command", () => {
  it("covers the frontend commands in InputLine.svelte", () => {
    const found = scanInputLine();
    expect(found.size, "the scan found no commands — the regex has rotted").toBeGreaterThan(5);

    const k = known();
    const missing = [...found].filter((c) => !k.has(c));
    expect(missing, `dispatched in InputLine.svelte but absent from COMMANDS: ${missing.join(", ")}`).toEqual([]);
  });

  it("covers the core commands in handleLocalCommand", () => {
    const found = scanClientGo();
    expect(found.size, "the scan found no commands — the regex has rotted").toBeGreaterThan(3);

    const k = known();
    const missing = [...found].filter((c) => !k.has(c));
    expect(missing, `dispatched in client.go but absent from COMMANDS: ${missing.join(", ")}`).toEqual([]);
  });

  // The two per-scan tests above each guard against total rot (the regex
  // matching nothing) with a headroom-padded minimum count, so a scan that
  // still finds most of its commands passes silently — extracting the /play
  // family into its own module would leave InputLine at 7 > 5 and four
  // commands would go unscanned forever. This assertion has no headroom: the
  // union of both scans must equal the catalog's full set of names and
  // aliases, exactly, in both directions. A command dispatched somewhere but
  // missing from COMMANDS fails here even if the per-scan minimum still
  // passes; a catalog entry for a command dispatched nowhere (a deleted
  // command still being advertised) fails here too, which the per-scan tests
  // cannot see at all since they only check dispatched -> catalogued.
  it("the union of both scans exactly equals the catalog's names and aliases", () => {
    const dispatched = new Set([...scanInputLine(), ...scanClientGo()]);
    const catalogued = known();

    const uncatalogued = [...dispatched].filter((c) => !catalogued.has(c));
    const undispatched = [...catalogued].filter((c) => !dispatched.has(c));

    expect(
      uncatalogued,
      `dispatched but not in COMMANDS (add a catalog entry): ${uncatalogued.join(", ")}`,
    ).toEqual([]);
    expect(
      undispatched,
      `in COMMANDS but not dispatched anywhere (remove the stale entry, or the scan missed it): ${undispatched.join(", ")}`,
    ).toEqual([]);
  });
});
