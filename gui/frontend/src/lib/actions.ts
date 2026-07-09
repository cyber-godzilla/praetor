// Wrap an index by delta within [0, len), used by the Actions tab set cycler so
// prev/next roll over (0 -> 1 -> 2 -> 0). Returns 0 for an empty list.
export function cycleIndex(current: number, delta: number, len: number): number {
  if (len <= 0) return 0;
  return (current + delta + len) % len;
}
