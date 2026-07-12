import { describe, it, expect } from "vitest";
import { FOLLOW_BAND_LINES, followBandPx, gapToBottom, withinBand } from "./scroll";

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
