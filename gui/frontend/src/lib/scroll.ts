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

// Decide the follow state after a scroll event.
//
// - Within the band → follow (re-engage). Scrolling back near the bottom, or a
//   small scroll within the band, keeps the tail followed.
// - Outside the band, only DISENGAGE on a genuine upward scroll (scrollTop
//   decreased). This is the crucial burst guard: a fast append grows the gap
//   without moving scrollTop up, so it must not flip following off — otherwise
//   large chunks of text leave the view stuck behind the tail.
// - Otherwise keep the current state.
export function nextAutoFollow(s: {
  gapPx: number;
  bandPx: number;
  top: number;
  lastTop: number;
  current: boolean;
}): boolean {
  if (withinBand(s.gapPx, s.bandPx)) return true;
  if (s.top < s.lastTop) return false;
  return s.current;
}

// Size and position of the custom scrollbar thumb. The pane's native scrollbar
// is hidden so the text keeps full height; this drives an overlaid rail thumb
// instead. `sizePx`/`offsetPx` are in track pixels; `scrollable` is false when
// the content fits (no thumb needed).
export function thumbMetrics(m: {
  scrollTop: number;
  scrollHeight: number;
  clientHeight: number;
  trackPx: number;
  minThumbPx: number;
}): { sizePx: number; offsetPx: number; scrollable: boolean } {
  const maxScroll = Math.max(0, m.scrollHeight - m.clientHeight);
  if (maxScroll <= 0 || m.trackPx <= 0 || m.scrollHeight <= 0) {
    return { sizePx: m.trackPx, offsetPx: 0, scrollable: false };
  }
  const raw = (m.clientHeight / m.scrollHeight) * m.trackPx;
  const sizePx = Math.min(m.trackPx, Math.max(m.minThumbPx, raw));
  const travel = m.trackPx - sizePx;
  const offsetPx = travel * (m.scrollTop / maxScroll);
  return { sizePx, offsetPx, scrollable: true };
}

// scrollTop delta for a thumb drag of `dyPx` pixels, mapping thumb travel back
// to content scroll. `thumbPx` is the current thumb size.
export function scrollDeltaForThumbDrag(m: {
  dyPx: number;
  trackPx: number;
  thumbPx: number;
  scrollHeight: number;
  clientHeight: number;
}): number {
  const maxScroll = Math.max(0, m.scrollHeight - m.clientHeight);
  const travel = m.trackPx - m.thumbPx;
  if (travel <= 0) return 0;
  return (m.dyPx * maxScroll) / travel;
}
