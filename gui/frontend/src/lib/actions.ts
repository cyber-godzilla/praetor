// Wrap an index by delta within [0, len), used by the Actions tab set cycler so
// prev/next roll over (0 -> 1 -> 2 -> 0). Returns 0 for an empty list.
export function cycleIndex(current: number, delta: number, len: number): number {
  if (len <= 0) return 0;
  return (current + delta + len) % len;
}

// Action Set identity must survive rename and reorder. The backend also repairs
// missing IDs from legacy or hand-edited configurations, but new frontend
// entries receive their identity before the first save so local state and the
// persisted representation agree immediately.
export function newActionSetID(): string {
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  // RFC 4122 variant/version bits make the opaque suffix familiar and keep
  // this usable in webviews that expose getRandomValues but not randomUUID.
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return `action-${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
