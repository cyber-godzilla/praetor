import { describe, expect, it } from "vitest";
import cases from "../../../../internal/variables/testdata/conformance.json";
import { expandVariables, VariableExpansionError } from "./variables";

describe("variable expansion conformance", () => {
  for (const test of cases) {
    it(test.name, () => {
      if (test.error) {
        try {
          expandVariables(test.input, test.values as Record<string, string>);
          throw new Error("expected expansion to fail");
        } catch (error) {
          expect(error).toBeInstanceOf(VariableExpansionError);
          expect((error as VariableExpansionError).code).toBe(test.error);
          expect((error as VariableExpansionError).variable).toBe(test.errorName);
        }
      } else {
        expect(expandVariables(test.input, test.values as Record<string, string>)).toBe(test.output);
      }
    });
  }
});
