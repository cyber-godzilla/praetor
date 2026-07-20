// Reverse history search (Ctrl+R) helpers, readline-style: scan the command
// history newest-first for entries containing the query.

// searchBackward returns the index of the newest history entry at or before
// `from` whose text contains `query` (case-insensitive), or -1. An empty query
// never matches.
export function searchBackward(history: string[], query: string, from: number): number {
  const q = query.toLowerCase();
  if (!q) return -1;
  for (let i = Math.min(from, history.length - 1); i >= 0; i--) {
    if (history[i].toLowerCase().includes(q)) return i;
  }
  return -1;
}

// dropLastChar removes the final user-perceived character, handling surrogate
// pairs so emoji and other astral-plane characters aren't torn in half.
export function dropLastChar(s: string): string {
  const chars = Array.from(s);
  chars.pop();
  return chars.join("");
}
