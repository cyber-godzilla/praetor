// centerInBox centers `text` within a fixed `width`-column field, padding with
// spaces on both sides. Text longer than the field is clipped so the splash
// screen's ASCII-art border can never be pushed out of alignment.
//
// The version string used to be hard-truncated to 6 chars to fit the box, which
// silently dropped a digit once the patch number reached two digits (e.g.
// "v0.2.10" rendered as "v0.2.1"). Centering in the full interior width fixes
// that for any version length.
export function centerInBox(text: string, width: number): string {
  const t = text.length > width ? text.slice(0, width) : text;
  const total = width - t.length;
  const left = Math.floor(total / 2);
  return " ".repeat(left) + t + " ".repeat(total - left);
}
