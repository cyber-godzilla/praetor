// Pure parsing/formatting for the GUI-only /notes command family. No DOM or
// bridge access, so it is unit-testable in isolation.

export type NotesCommand =
  | { kind: "open-list" }
  | { kind: "new"; title: string }
  | { kind: "open"; title: string }
  | { kind: "delete"; title: string }
  | { kind: "list" }
  | { kind: "usage" };

// parseNotesCommand parses the text AFTER "/notes" (already stripped/trimmed by
// the caller). The subcommand is case-insensitive; the title keeps its case.
export function parseNotesCommand(rest: string): NotesCommand {
  const trimmed = rest.trim();
  if (trimmed === "") return { kind: "open-list" };
  const m = trimmed.match(/^(\S+)(?:\s+(.*))?$/);
  const sub = (m?.[1] ?? "").toLowerCase();
  const arg = (m?.[2] ?? "").trim();
  switch (sub) {
    case "add":
      return { kind: "new", title: arg };
    case "open":
      return arg ? { kind: "open", title: arg } : { kind: "usage" };
    case "delete":
      return arg ? { kind: "delete", title: arg } : { kind: "usage" };
    case "list":
      return { kind: "list" };
    default:
      return { kind: "usage" };
  }
}

// formatNotesList renders the `/notes list` output lines: "Title — preview",
// or just the title when there is no body preview.
export function formatNotesList(items: { title: string; preview: string }[]): string[] {
  if (items.length === 0) return ["No notes."];
  return items.map((n) => (n.preview ? `${n.title} — ${n.preview}` : n.title));
}
