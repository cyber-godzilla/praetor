import { describe, expect, it } from "vitest";
import cases from "../../../../internal/wildcard/testdata/conformance.json";
import { compileWildcard, type MatchScope } from "./wildcard";

describe("wildcard conformance", () => {
  for (const test of cases) {
    it(test.name, () => {
      const pattern = compileWildcard(test.pattern, test.scope as MatchScope);
      expect(pattern.matches(test.text)).toBe(test.match);
      expect(pattern.findAll(test.text).map((span) => test.text.slice(span.start, span.end))).toEqual(test.spans);
    });
  }
});
