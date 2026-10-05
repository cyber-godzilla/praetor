import { describe, it, expect } from "vitest";
import {
  COMMANDS,
  matchCommands,
  completionFor,
  tabComplete,
  type CommandSpec,
} from "./commands";
import { isAllowedDuringPlay } from "./playcmd";

const names = (r: ReturnType<typeof matchCommands>) => r.map((c) => c.name);

describe("COMMANDS", () => {
  it("has no duplicate names or aliases", () => {
    const seen = new Set<string>();
    for (const c of COMMANDS) {
      for (const n of [c.name, ...(c.aliases ?? [])]) {
        expect(seen.has(n), `${n} appears twice`).toBe(false);
        seen.add(n);
      }
    }
  });

  it("every entry starts with a slash and has a description", () => {
    for (const c of COMMANDS) {
      expect(c.name.startsWith("/")).toBe(true);
      for (const a of c.aliases ?? []) expect(a.startsWith("/")).toBe(true);
      expect(c.desc.length).toBeGreaterThan(0);
    }
  });
});

describe("matchCommands", () => {
  it("returns nothing for ordinary game text", () => {
    expect(matchCommands("look")).toEqual([]);
    expect(matchCommands("")).toEqual([]);
    expect(matchCommands("say /play is fun")).toEqual([]);
  });

  it("lists everything for a bare slash", () => {
    expect(matchCommands("/")).toHaveLength(COMMANDS.length);
  });

  it("narrows by prefix", () => {
    expect(names(matchCommands("/pl"))).toEqual(["/play"]);
    expect(names(matchCommands("/p"))).toEqual(
      expect.arrayContaining(["/play", "/pause"]),
    );
  });

  it("matches case-insensitively", () => {
    expect(names(matchCommands("/PL"))).toEqual(["/play"]);
  });

  it("matches an alias without listing it as a separate entry", () => {
    const r = matchCommands("/sm");
    expect(names(r)).toEqual(["/mode"]);
    expect(r).toHaveLength(1);
  });

  // The signature must stay visible while arguments are typed — that is the
  // whole point of the hint.
  it("keeps the exact match once a space is typed", () => {
    expect(names(matchCommands("/mode "))).toEqual(["/mode"]);
    expect(names(matchCommands("/mode aggro"))).toEqual(["/mode"]);
    expect(names(matchCommands("/sm aggro"))).toEqual(["/mode"]);
  });

  it("returns nothing for an unknown command with arguments", () => {
    expect(matchCommands("/nope ")).toEqual([]);
    expect(matchCommands("/nope arg")).toEqual([]);
  });

  it("returns nothing when the input has a newline (a pasted block is prose)", () => {
    expect(matchCommands("/play\nlook")).toEqual([]);
  });

  it("filters to the control commands during a performance", () => {
    expect(names(matchCommands("/", { playing: true })).sort()).toEqual(
      ["/next", "/pause", "/resume", "/stop"].sort(),
    );
    expect(matchCommands("/mode", { playing: true })).toEqual([]);
    expect(names(matchCommands("/pa", { playing: true }))).toEqual(["/pause"]);
  });

  // A command with no `args` dispatches on exact string equality, so trailing
  // text means it is no longer that command — showing its signature would
  // promise something the input will not do.
  it("rejects trailing text on a command that takes no arguments", () => {
    expect(matchCommands("/play foo")).toEqual([]);
    expect(matchCommands("/help x")).toEqual([]);
  });

  it("rejects trailing text on a niladic command during a performance", () => {
    expect(matchCommands("/stop x", { playing: true })).toEqual([]);
  });

  // Guard against over-correcting: commands that declare `args` still match
  // past the space, including through an alias.
  it("still matches commands that take arguments past the space", () => {
    expect(names(matchCommands("/mode aggro"))).toEqual(["/mode"]);
    expect(names(matchCommands("/notes list"))).toEqual(["/notes"]);
    expect(names(matchCommands("/sm aggro"))).toEqual(["/mode"]);
  });

  // Dispatch trims the input before checking it (InputLine does
  // `line.trim()`), so the matcher must see the same thing — otherwise the
  // hint hides while the command still works.
  it("tolerates leading whitespace, mirroring InputLine's trim before dispatch", () => {
    expect(names(matchCommands(" /play"))).toEqual(["/play"]);
    expect(names(matchCommands("  /pl"))).toEqual(["/play"]);
  });

  // Same trim, trailing side: a space after a niladic command still dispatches
  // (InputLine's `line.trim()` strips it before comparing), so the hint must
  // not disappear the instant the user presses space.
  it("tolerates trailing whitespace on a niladic command, mirroring InputLine's trim", () => {
    expect(names(matchCommands("/play "))).toEqual(["/play"]);
    expect(names(matchCommands("/help "))).toEqual(["/help"]);
    expect(names(matchCommands(" /play "))).toEqual(["/play"]);
  });

  // Guard against over-correcting the trailing-whitespace fix: real trailing
  // text (not just whitespace) on a niladic command is still rejected, and a
  // command that takes arguments still matches past its trailing space.
  it("still rejects real trailing text on a niladic command after the trim fix", () => {
    expect(matchCommands("/play foo")).toEqual([]);
  });
  it("still matches a command with arguments through trailing whitespace", () => {
    expect(names(matchCommands("/mode aggro "))).toEqual(["/mode"]);
  });

  it("still returns nothing for a newline even with the leading-space trim", () => {
    expect(matchCommands("/play\nlook")).toEqual([]);
  });

  // playcmd.ALLOWED and this catalog's DURING_PLAY both enumerate the same
  // four control commands independently — nothing else pins them together, so
  // this proves they still agree in both directions: everything the hint
  // shows during a performance is accepted by the input lockout, and nothing
  // else in the catalog is secretly accepted that the hint doesn't show.
  it("agrees with playcmd's isAllowedDuringPlay on the play-control commands", () => {
    const shown = matchCommands("/", { playing: true });
    for (const c of shown) {
      expect(
        isAllowedDuringPlay(c.name),
        `${c.name} is shown during a performance but rejected by isAllowedDuringPlay`,
      ).toBe(true);
    }

    const shownNames = shown.map((c) => c.name).sort();
    const alsoAllowed = COMMANDS.filter((c) => isAllowedDuringPlay(c.name))
      .map((c) => c.name)
      .sort();
    expect(alsoAllowed, "isAllowedDuringPlay accepts a catalog command the hint doesn't show during a performance").toEqual(
      shownNames,
    );
  });
});

describe("matchCommands with mode specs", () => {
  const MODES = [
    { name: "disable", usage: "", desc: "", chains: false },
    { name: "lock_job", usage: "<citizen|trader>", desc: "Run a lock job", chains: false },
    { name: "locksmith", usage: "[skip:<n>]", desc: "Unjam containers", chains: true },
    { name: "loot", usage: "<item> [corpse#]", desc: "Loot corpses", chains: true },
    { name: "macro", usage: "[nokill]", desc: "Attack rotation", chains: false },
  ];
  const m = (input: string) => matchCommands(input, { modes: MODES });

  it("offers every mode once the command is committed", () => {
    expect(names(m("/mode "))).toEqual([
      "/mode disable",
      "/mode lock_job",
      "/mode locksmith",
      "/mode loot",
      "/mode macro",
    ]);
  });

  it("narrows the offer by mode-name prefix", () => {
    expect(names(m("/mode lo"))).toEqual([
      "/mode lock_job",
      "/mode locksmith",
      "/mode loot",
    ]);
  });

  it("shows the resolved mode's own signature and description", () => {
    const r = m("/mode macro");
    expect(r).toHaveLength(1);
    expect(r[0].name).toBe("/mode macro");
    expect(r[0].args).toBe("[nokill]");
    expect(r[0].desc).toBe("Attack rotation");
  });

  it("appends the after: token only for a chaining mode", () => {
    expect(m("/mode loot")[0].args).toBe("<item> [corpse#] [after:<mode>]");
    expect(m("/mode macro")[0].args).toBe("[nokill]");
  });

  it("shows after: alone when a chaining mode takes no other arguments", () => {
    const chainOnly = [{ name: "idle", usage: "", desc: "Rest", chains: true }];
    const r = matchCommands("/mode idle", { modes: chainOnly });
    expect(r[0].args).toBe("[after:<mode>]");
  });

  it("keeps the resolved mode up while its arguments are typed", () => {
    const r = m("/mode loot bronze|alanti");
    expect(r).toHaveLength(1);
    expect(r[0].name).toBe("/mode loot");
  });

  it("resolves the mode case-insensitively, like the core does", () => {
    expect(names(m("/mode LOOT"))).toEqual(["/mode loot"]);
  });

  it("labels rows with the alias the user actually typed", () => {
    expect(names(m("/sm loot"))).toEqual(["/sm loot"]);
  });

  it("falls back to the generic signature for an unknown mode", () => {
    const r = m("/mode nosuchmode");
    expect(names(r)).toEqual(["/mode"]);
    expect(r[0].args).toBe("<name> [args…]");
  });

  it("offers an undeclared mode by name with nothing to promise", () => {
    const r = m("/mode disable");
    expect(names(r)).toEqual(["/mode disable"]);
    expect(r[0].args).toBeUndefined();
    expect(r[0].desc).toBe("");
  });

  it("leaves every other command untouched", () => {
    expect(names(m("/loot"))).toEqual([]);
    expect(names(m("/wiki foo"))).toEqual(["/wiki"]);
    expect(names(m("/mo"))).toEqual(["/mode"]);
  });

  it("never offers a hidden mode while browsing", () => {
    const withHidden = [
      { name: "leg_one", usage: "", desc: "Route leg", chains: true, hidden: true },
      { name: "loot", usage: "<item>", desc: "Loot corpses", chains: true },
    ];
    expect(names(matchCommands("/mode ", { modes: withHidden }))).toEqual([
      "/mode loot",
    ]);
    expect(names(matchCommands("/mode le", { modes: withHidden }))).toEqual([
      "/mode",
    ]);
  });

  // Silence would itself leak that the mode exists: an unknown name shows the
  // generic row, so a hidden one must too.
  it("treats a fully typed hidden mode exactly like an unknown one", () => {
    const withHidden = [
      { name: "leg_one", usage: "", desc: "Route leg", chains: true, hidden: true },
    ];
    const hiddenRow = matchCommands("/mode leg_one", { modes: withHidden });
    const unknownRow = matchCommands("/mode nosuchmode", { modes: withHidden });
    expect(hiddenRow).toEqual(unknownRow);
    expect(names(hiddenRow)).toEqual(["/mode"]);

    // ...including once its arguments are being typed.
    expect(names(matchCommands("/mode leg_one after:disable", { modes: withHidden })))
      .toEqual(["/mode"]);
  });

  it("still offers a mode that declares hidden false", () => {
    const shown = [{ name: "loot", usage: "<item>", desc: "Loot", chains: false, hidden: false }];
    expect(names(matchCommands("/mode lo", { modes: shown }))).toEqual(["/mode loot"]);
  });

  it("behaves exactly as before when no specs are loaded", () => {
    expect(names(matchCommands("/mode loot"))).toEqual(["/mode"]);
    expect(names(matchCommands("/mode loot", { modes: [] }))).toEqual(["/mode"]);
  });
});

describe("completionFor", () => {
  const mode = COMMANDS.find((c) => c.name === "/mode")!;
  const help = COMMANDS.find((c) => c.name === "/help")!;
  const set = COMMANDS.find((c) => c.name === "/set")!;
  const berserk: CommandSpec = { name: "/mode berserk", desc: "" };
  const smBerserk: CommandSpec = { name: "/sm berserk", desc: "" };

  it("extends a partial command name", () => {
    expect(completionFor("/m", mode)).toBe("/mode ");
    expect(completionFor("/he", help)).toBe("/help ");
  });

  it("completes to the alias being typed, not the canonical name", () => {
    expect(completionFor("/s", mode)).toBe("/sm ");
    expect(completionFor("/sm", mode)).toBe("/sm ");
  });

  it("completes a mode row from the command or a partial name", () => {
    expect(completionFor("/mode ", berserk)).toBe("/mode berserk ");
    expect(completionFor("/mode ber", berserk)).toBe("/mode berserk ");
    expect(completionFor("/sm ber", smBerserk)).toBe("/sm berserk ");
  });

  it("matches case-insensitively and completes to canonical casing", () => {
    expect(completionFor("/MODE ber", berserk)).toBe("/mode berserk ");
  });

  it("adds only the trailing space on an exact match", () => {
    expect(completionFor("/mode berserk", berserk)).toBe("/mode berserk ");
    expect(completionFor("/help", help)).toBe("/help ");
  });

  it("returns null for a row that could only shorten the input", () => {
    expect(completionFor("/mode berserk stand=true", berserk)).toBeNull();
    expect(completionFor("/set health 20", set)).toBeNull();
  });

  it("ignores surrounding whitespace the way dispatch does", () => {
    expect(completionFor("  /he  ", help)).toBe("/help ");
  });
});

describe("tabComplete", () => {
  const berserk: CommandSpec = { name: "/mode berserk", desc: "" };
  const bersark: CommandSpec = { name: "/mode bersark", desc: "" };
  const fish: CommandSpec = { name: "/mode fish", desc: "" };
  const fishing: CommandSpec = { name: "/mode fishing", desc: "" };

  it("completes a unique match in full, with its trailing space", () => {
    expect(tabComplete("/he", matchCommands("/he"))).toBe("/help ");
    expect(tabComplete("/mode ber", [berserk])).toBe("/mode berserk ");
  });

  it("advances to the shared prefix without a trailing space", () => {
    expect(tabComplete("/mode ber", [berserk, bersark])).toBe("/mode bers");
  });

  it("stops at the shared prefix when one candidate extends another", () => {
    expect(tabComplete("/mode fi", [fish, fishing])).toBe("/mode fish");
  });

  it("returns null when the shared prefix is already typed", () => {
    expect(tabComplete("/mode bers", [berserk, bersark])).toBeNull();
    // "/s" reaches /send, /set, /stop and /mode-via-/sm; they share nothing more.
    expect(tabComplete("/s", matchCommands("/s"))).toBeNull();
  });

  it("returns null when nothing matches", () => {
    expect(tabComplete("/mode ber", [])).toBeNull();
    expect(tabComplete("/mode berserk stand=true", [berserk])).toBeNull();
  });
});
