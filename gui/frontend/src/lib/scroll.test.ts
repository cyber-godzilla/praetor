import { describe, it, expect } from "vitest";
import {
  FOLLOW_BAND_LINES,
  followBandPx,
  gapToBottom,
  withinBand,
  nextAutoFollow,
  thumbMetrics,
  scrollDeltaForThumbDrag,
} from "./scroll";

describe("followBandPx", () => {
  it("is 25 lines tall at line-height 1.4", () => {
    // 14px font → 25 * 14 * 1.4 = 490px
    expect(followBandPx(14)).toBeCloseTo(490);
    expect(followBandPx(20)).toBeCloseTo(700);
  });
  it("scales with font size", () => {
    expect(followBandPx(28)).toBeCloseTo(2 * followBandPx(14));
  });
  it("uses the documented 25-line band", () => {
    expect(FOLLOW_BAND_LINES).toBe(25);
  });
});

describe("gapToBottom", () => {
  it("is zero at the very bottom", () => {
    expect(gapToBottom({ scrollHeight: 1000, scrollTop: 800, clientHeight: 200 })).toBe(0);
  });
  it("grows as you scroll up", () => {
    expect(gapToBottom({ scrollHeight: 1000, scrollTop: 300, clientHeight: 200 })).toBe(500);
  });
});

describe("withinBand", () => {
  const band = followBandPx(14); // 490px

  it("follows at the exact bottom", () => {
    expect(withinBand(0, band)).toBe(true);
  });
  it("follows a few pixels short of the bottom (fixes the momentum-scroll bug)", () => {
    expect(withinBand(12, band)).toBe(true);
  });
  it("follows at the band edge", () => {
    expect(withinBand(band, band)).toBe(true);
  });
  it("detaches just past the band", () => {
    expect(withinBand(band + 1, band)).toBe(false);
  });
  it("detaches far from the bottom", () => {
    expect(withinBand(5000, band)).toBe(false);
  });
});

describe("nextAutoFollow", () => {
  const band = followBandPx(14); // 490px

  it("follows when within the band, regardless of prior state", () => {
    expect(nextAutoFollow({ gapPx: 0, bandPx: band, top: 100, lastTop: 900, current: false })).toBe(true);
    expect(nextAutoFollow({ gapPx: 200, bandPx: band, top: 900, lastTop: 100, current: false })).toBe(true);
  });

  it("detaches when the user scrolls up out of the band", () => {
    // scrollTop decreased (700 < 1000) and we're now far from the bottom.
    expect(nextAutoFollow({ gapPx: 3000, bandPx: band, top: 700, lastTop: 1000, current: true })).toBe(false);
  });

  it("does NOT detach on a burst: gap grew but scrollTop did not move up", () => {
    // The programmatic scroll-to-bottom event fires after more content grew the
    // gap past the band; scrollTop is unchanged (top === lastTop). Must keep
    // following so large chunks don't leave the view stuck behind.
    expect(nextAutoFollow({ gapPx: 3000, bandPx: band, top: 5000, lastTop: 5000, current: true })).toBe(true);
  });

  it("does NOT detach when scrollTop increased (scrolling toward the bottom)", () => {
    expect(nextAutoFollow({ gapPx: 3000, bandPx: band, top: 1200, lastTop: 1000, current: true })).toBe(true);
  });

  it("stays detached while paging down but still far from the bottom", () => {
    // Already detached, moving down (top increased) but not yet within band.
    expect(nextAutoFollow({ gapPx: 2000, bandPx: band, top: 1500, lastTop: 1000, current: false })).toBe(false);
  });
});

describe("thumbMetrics", () => {
  const min = 24;

  it("reports not-scrollable when content fits", () => {
    const m = thumbMetrics({ scrollTop: 0, scrollHeight: 300, clientHeight: 300, trackPx: 300, minThumbPx: min });
    expect(m.scrollable).toBe(false);
  });

  it("sizes the thumb proportionally to the visible fraction", () => {
    // Half the content visible → thumb is half the track.
    const m = thumbMetrics({ scrollTop: 0, scrollHeight: 1000, clientHeight: 500, trackPx: 400, minThumbPx: min });
    expect(m.scrollable).toBe(true);
    expect(m.sizePx).toBeCloseTo(200);
    expect(m.offsetPx).toBeCloseTo(0);
  });

  it("puts the thumb at the bottom of the track when scrolled to the end", () => {
    const m = thumbMetrics({ scrollTop: 500, scrollHeight: 1000, clientHeight: 500, trackPx: 400, minThumbPx: min });
    expect(m.offsetPx).toBeCloseTo(400 - m.sizePx); // travel fully used
  });

  it("clamps the thumb to a minimum size for very long buffers", () => {
    const m = thumbMetrics({ scrollTop: 0, scrollHeight: 100000, clientHeight: 500, trackPx: 400, minThumbPx: min });
    expect(m.sizePx).toBe(min);
  });
});

describe("scrollDeltaForThumbDrag", () => {
  it("maps thumb travel to full content scroll", () => {
    // track 400, thumb 200 → travel 200; dragging the full travel scrolls the
    // full maxScroll (500).
    const d = scrollDeltaForThumbDrag({ dyPx: 200, trackPx: 400, thumbPx: 200, scrollHeight: 1000, clientHeight: 500 });
    expect(d).toBeCloseTo(500);
  });
  it("scales a partial drag proportionally", () => {
    const d = scrollDeltaForThumbDrag({ dyPx: 50, trackPx: 400, thumbPx: 200, scrollHeight: 1000, clientHeight: 500 });
    expect(d).toBeCloseTo(125);
  });
  it("is zero when there is no travel", () => {
    expect(scrollDeltaForThumbDrag({ dyPx: 100, trackPx: 200, thumbPx: 200, scrollHeight: 1000, clientHeight: 500 })).toBe(0);
  });
});
