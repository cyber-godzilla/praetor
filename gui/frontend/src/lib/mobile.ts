// The layout query matches GameView/MobileDock exactly. Coarse pointers also
// opt into command-keyboard behavior, but do not turn a desktop-width layout
// into the mobile dock.
export const MOBILE_LAYOUT_QUERY = "(max-width: 899px)";
export const MOBILE_COMMAND_QUERY = `${MOBILE_LAYOUT_QUERY}, (pointer: coarse)`;

export interface OuterPageScrollMetrics {
  windowY: number;
  scrollingElementTop: number;
  documentElementTop: number;
  bodyTop: number;
  visualViewportOffsetTop: number;
}

// Only the responsive mobile web layout uses its compact output preference.
// Desktop-width browsers and the native Wails shell retain the desktop value.
export function outputFontSizeForLayout(
  desktopSize: number,
  mobileSize: number,
  mobileWebLayout: boolean,
): number {
  return mobileWebLayout ? mobileSize : desktopSize;
}

// Mobile browsers may pan the layout or visual viewport while bringing up the
// software keyboard. Ignore sub-pixel rounding, but treat movement in any of
// the browser-owned outer scroll positions as page drift. This deliberately
// excludes the session output pane, which owns its own independent scrollTop.
export function outerPageHasVerticalDrift(metrics: OuterPageScrollMetrics): boolean {
  return [
    metrics.windowY,
    metrics.scrollingElementTop,
    metrics.documentElementTop,
    metrics.bodyTop,
    metrics.visualViewportOffsetTop,
  ].some((value) => Number.isFinite(value) && value > 1);
}

// lowercaseFirstCommandLetter changes only the first Unicode character that
// has case. Leading whitespace, slash-command punctuation, numbers, and emoji
// are preserved, as is every character after the first letter.
export function lowercaseFirstCommandLetter(value: string): string {
  const characters = Array.from(value);
  for (let i = 0; i < characters.length; i++) {
    const lower = characters[i].toLowerCase();
    const upper = characters[i].toUpperCase();
    if (lower === upper) continue;
    if (characters[i] === lower) return value;
    characters[i] = lower;
    return characters.join("");
  }
  return value;
}
