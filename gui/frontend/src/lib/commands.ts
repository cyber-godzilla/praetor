import type { ModeSpec } from "./types";

// The single catalog of slash commands, shared by the input hint and /help.
//
// It exists because those two drifted: /help carried a hand-maintained list
// that silently went four features out of date. The catalog is display-only —
// dispatch still lives in InputLine.svelte (frontend commands) and
// internal/client/client.go (core commands, shared with the TUI). A
// source-scanning test guards against an entry going missing.
//
// A command-registration pattern that folds dispatch and metadata into one
// entry is a planned pre-1.0 cleanup (todo/p2-command-registration-pattern.md);
// CommandSpec is shaped to become its metadata half.

export interface CommandSpec {
  name: string; // "/mode"
  aliases?: string[]; // ["/sm"] — matched and displayed, never a separate row
  args?: string; // "<name> [args…]" — omitted when the command takes none
  desc: string; // one line
}

// The token a chaining mode accepts in any argument position. Rendered by the
// hint rather than stored in each mode's usage string, so the convention lives
// in exactly one place. Modes opt in with `chains`.
const AFTER_TOKEN = "[after:<mode>]";

// modeRow renders one mode as a hint row. `token` is the command the user
// actually typed (/mode or /sm), so the row reads back as the line being built.
function modeRow(token: string, m: ModeSpec): CommandSpec {
  const args = [m.usage?.trim(), m.chains ? AFTER_TOKEN : ""]
    .filter(Boolean)
    .join(" ");
  return {
    name: `${token} ${m.name}`,
    args: args || undefined,
    desc: m.desc ?? "",
  };
}

// modeRows resolves the text after "/mode " against the loaded modes. Returns
// [] when nothing matches, which the caller treats as "fall back to the generic
// signature" — a typo should not silently promise a mode that does not exist.
function modeRows(rest: string, token: string, modes: ModeSpec[]): CommandSpec[] {
  rest = rest.replace(/^\s+/, "");
  const nextSpace = rest.search(/\s/);

  // Hidden modes are invisible to the hint at every stage, including once the
  // name is typed in full. Revealing one on an exact match would defeat the
  // flag, and going silent instead of falling through to the generic row would
  // leak its existence — an unknown name shows that row, so a hidden name must
  // look identical.
  const offerable = modes.filter((m) => !m.hidden);

  // A space after the mode name settles it: the player has moved on to the
  // arguments, so stop offering alternatives and keep that one signature up.
  if (nextSpace !== -1) {
    const typed = rest.slice(0, nextSpace).toLowerCase();
    const exact = offerable.find((m) => m.name.toLowerCase() === typed);
    return exact ? [modeRow(token, exact)] : [];
  }

  const prefix = rest.toLowerCase();
  return offerable
    .filter((m) => m.name.toLowerCase().startsWith(prefix))
    .map((m) => modeRow(token, m));
}

// Commands accepted while a /play performance is running. Everything else is
// rejected by the input lockout, so the hint filters to these.
const DURING_PLAY = ["/pause", "/resume", "/stop", "/next"];

export const COMMANDS: CommandSpec[] = [
  { name: "/help", desc: "Show key bindings and commands" },
  { name: "/guide", desc: "Open the Praetor wiki links" },
  { name: "/list", desc: "List available Lua modes" },
  { name: "/mode", aliases: ["/sm"], args: "<name> [args…]", desc: "Switch Lua mode" },
  { name: "/toggle", args: "<label>", desc: "Toggle a mode state value" },
  { name: "/set", args: "<label> <value>", desc: "Set a mode state value" },
  { name: "/calc", aliases: ["/rb"], desc: "Rank-bonus calculator" },
  { name: "/wiki", args: "[name]", desc: "Open a wiki bookmark, or list them" },
  { name: "/maps", args: "[name]", desc: "Open a map bookmark, or list them" },
  { name: "/kudos", args: "[name] [message]", desc: "Kudos menu, add a favourite, or queue one" },
  { name: "/notes", args: "[add|open|delete|list] [title]", desc: "Notepad" },
  { name: "/send", desc: "Pick a text file and send it to the game" },
  { name: "/play", desc: "Pick a script and perform it" },
  { name: "/pause", desc: "Hold the running performance" },
  { name: "/resume", desc: "Continue a held performance" },
  { name: "/stop", desc: "End the performance and drop its state" },
  { name: "/next", desc: "Release a %wait-key hold" },
];

// commandHead formats a command's name with its aliases parenthesized —
// "/mode (/sm)" — the one place CommandHint and HelpModal agree on how a
// command is named; each still renders its own argument signature.
export function commandHead(c: CommandSpec): string {
  const names = [c.name, ...(c.aliases ?? [])];
  return names.length > 1 ? `${names[0]} (${names.slice(1).join(", ")})` : names[0];
}

// completionFor returns the text a hint row inserts when it is chosen, or null
// when that row cannot complete what is typed.
//
// The prefix test is the whole safety story. A non-null result always *extends*
// the input, so filling can never delete a character the player entered. That is
// why a row merely echoing an already-typed command's signature ("/mode berserk
// stand=true") comes back null and renders inert: it could only shorten the
// line, and clicking it would eat the arguments.
//
// Aliases complete to the name being typed — "/s" reaches /mode through "/sm",
// and inserting "/mode " there would rewrite the line out from under the player.
// commandHead()'s "/mode (/sm)" is display-only and never insertable.
//
// Mode rows need no special case: modeRow already builds `name` as the full
// completed text ("/mode berserk", or "/sm berserk" when the alias was typed).
export function completionFor(input: string, c: CommandSpec): string | null {
  // Trim to match dispatch, which trims before it interprets the line.
  const typed = input.trim().toLowerCase();
  for (const n of [c.name, ...(c.aliases ?? [])]) {
    if (n.toLowerCase().startsWith(typed)) return n + " ";
  }
  return null;
}

// tabComplete returns the text Tab inserts, or null when Tab should change
// nothing. Readline semantics: a unique match completes in full, several matches
// advance to their shared prefix and stop, letting the narrowed list guide the
// next keystroke or click.
//
// The shared prefix comes back WITHOUT a trailing space — it is a partial word,
// not a finished token, and a space would falsely commit it.
//
// Every candidate extends the input (see completionFor), so their common prefix
// does too. That is what makes the length test below a sound "did we make
// progress" check, and it is why Tab can no more lose text than a click can.
export function tabComplete(input: string, matches: CommandSpec[]): string | null {
  const cands = matches
    .map((c) => completionFor(input, c))
    .filter((s): s is string => s !== null);
  if (cands.length === 0) return null;
  if (cands.length === 1) return cands[0];

  let lcp = cands[0];
  for (const s of cands.slice(1)) {
    let i = 0;
    while (i < lcp.length && i < s.length && lcp[i] === s[i]) i++;
    lcp = lcp.slice(0, i);
  }
  // No progress: the shared prefix is already typed, so there is nothing Tab can
  // add. "/s" is the everyday case — /send, /set, /stop and /sm share nothing
  // past it. The list is already on screen doing the rest of the work.
  return lcp.length > input.trim().length ? lcp : null;
}

// matchCommands returns the catalog entries to show for the current input.
// Returns [] whenever the hint should not appear at all, so callers need only
// check the length.
export function matchCommands(
  input: string,
  opts?: { playing?: boolean; modes?: ModeSpec[] },
): CommandSpec[] {
  // A pasted multi-line block whose first line starts with "/" is prose, not a
  // command — it goes to the game verbatim, so hinting at it would mislead.
  if (input.includes("\n") || input.includes("\r")) return [];

  // InputLine trims the line before dispatch, so the matcher has to see what
  // dispatch sees — otherwise whitespace (leading, or trailing on a niladic
  // command) hides the hint on a command that Enter would still run.
  //
  // One exception, remembered before the trim: for a command that takes a name,
  // a trailing space is the moment the player is about to type it, and so the
  // best moment to offer the choices. Trimming alone cannot tell "/mode" from
  // "/mode ", and only the latter should list every mode.
  const awaitingArg = /\s$/.test(input);
  input = input.trim();
  if (!input.startsWith("/")) return [];

  const pool = opts?.playing
    ? COMMANDS.filter((c) => DURING_PLAY.includes(c.name))
    : COMMANDS;

  const spaceAt = input.indexOf(" ");
  const token = (spaceAt === -1 ? input : input.slice(0, spaceAt)).toLowerCase();
  const namesOf = (c: CommandSpec) => [c.name, ...(c.aliases ?? [])];

  // Past the first space the command is committed: show only an exact match, so
  // the signature stays up while the arguments are typed and a typo shows
  // nothing rather than a stale list. A command that takes no arguments is
  // dispatched on exact equality, so trailing text means it is no longer that
  // command — "/play foo" is not /play, it falls through and is silently
  // discarded. Showing its signature would promise something the input will
  // not do.
  if (spaceAt !== -1) {
    const exact = pool.filter(
      (c) => c.args !== undefined && namesOf(c).some((n) => n === token),
    );
    // /mode is the one command that can describe itself properly: swap its
    // generic signature for the loaded mode's own. An unresolvable name yields
    // no rows and falls through to the generic entry below.
    if (opts?.modes?.length && exact.some((c) => c.name === "/mode")) {
      const rows = modeRows(input.slice(spaceAt + 1), token, opts.modes);
      if (rows.length > 0) return rows;
    }
    return exact;
  }

  const byPrefix = pool.filter((c) => namesOf(c).some((n) => n.startsWith(token)));

  // "/mode " — command named exactly, name not started. Offer the whole corpus.
  if (
    awaitingArg &&
    opts?.modes?.length &&
    byPrefix.some((c) => c.name === "/mode" && namesOf(c).includes(token))
  ) {
    return modeRows("", token, opts.modes);
  }
  return byPrefix;
}
