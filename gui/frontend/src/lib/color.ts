// safeColor is a defense-in-depth guard for values that reach an inline `style`
// attribute. Even though the Go protocol layer already normalizes font colors to
// a strict hex form, the frontend sanitizes again before emitting `color:` /
// `background:` so a stray value can never smuggle extra CSS declarations
// (`#0;position:fixed;background:url(http://evil)`) into the style string.
//
// Accepts a hex color (#RGB / #RRGGBB / #RRGGBBAA) or a bare color keyword
// (letters only). Anything containing the characters an injection needs — `;`,
// `:`, `(`, whitespace — fails closed and returns "".
export function safeColor(c: string | undefined | null): string {
  if (!c) return "";
  if (/^#[0-9a-fA-F]{3,8}$/.test(c)) return c;
  if (/^[a-zA-Z]+$/.test(c)) return c;
  return "";
}
