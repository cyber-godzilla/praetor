// Pure geometry for the output pane's "follow the tail" autoscroll.
//
// The view follows new output whenever it sits within a band of the newest
// line; past that band, scrolling is user-controlled only. The band is
// expressed in lines and converted to pixels using the pane's font size and the
// CSS line-height (1.4, see OutputPane's .pane style). Keeping this logic here —
// free of the DOM — lets us unit test the decisions the component makes.

// Number of lines from the bottom within which autoscroll stays engaged.
export const FOLLOW_BAND_LINES = 25;

// Must match `line-height` on .pane in OutputPane.svelte.
export const LINE_HEIGHT_RATIO = 1.4;

// Pixel height of the follow band for a given font size.
export function followBandPx(fontSize: number): number {
  return FOLLOW_BAND_LINES * fontSize * LINE_HEIGHT_RATIO;
}

// Distance in pixels from the current scroll position to the bottom.
export function gapToBottom(m: {
  scrollHeight: number;
  scrollTop: number;
  clientHeight: number;
}): number {
  return m.scrollHeight - m.scrollTop - m.clientHeight;
}

// Whether the view is close enough to the newest line to keep following it.
// A few pixels short of the true bottom is deep inside the band, so momentum
// scrolling that lands just shy of the end still counts as "following".
export function withinBand(gapPx: number, bandPx: number): boolean {
  return gapPx <= bandPx;
}
