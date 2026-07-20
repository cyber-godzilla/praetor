// Pure logic for the output-pane scrollback search (Ctrl+F): match collection
// over rendered line text and wrap-around stepping between matches.

export interface SearchableLine {
  id: number;
  text: string;
}

// matchingLineIds returns the ids of lines whose text contains the query,
// case-insensitively, in buffer order (oldest first). An empty or
// whitespace-only query matches nothing.
export function matchingLineIds(lines: SearchableLine[], query: string): number[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];
  const out: number[] = [];
  for (const l of lines) {
    if (l.text.toLowerCase().includes(q)) out.push(l.id);
  }
  return out;
}

// stepIndex moves through `len` matches with wrap-around. A negative current
// index means "no selection yet" and lands on the newest match (the end of the
// list) regardless of direction — searching a log starts from the most recent
// text.
export function stepIndex(current: number, delta: number, len: number): number {
  if (len <= 0) return -1;
  if (current < 0 || current >= len) return len - 1;
  return (current + delta + len) % len;
}
