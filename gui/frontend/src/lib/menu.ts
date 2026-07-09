// Clamp a context-menu's top-left so the whole menu stays within the viewport.
export function clampMenuPosition(
  x: number,
  y: number,
  menuW: number,
  menuH: number,
  viewW: number,
  viewH: number,
): { x: number; y: number } {
  return {
    x: Math.max(0, Math.min(x, viewW - menuW)),
    y: Math.max(0, Math.min(y, viewH - menuH)),
  };
}
