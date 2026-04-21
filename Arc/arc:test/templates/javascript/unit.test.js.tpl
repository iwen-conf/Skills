import { describe, it, expect } from "vitest";
import { {{FUNCTION_NAME}} } from "{{MODULE_PATH}}";

describe("{{FUNCTION_NAME}}", () => {
  it("should return expected result for nominal input", () => {
    expect({{FUNCTION_NAME}}({{SAMPLE_INPUT}})).toBe({{EXPECTED_OUTPUT}});
  });

  it("should handle boundary min", () => {
    expect({{FUNCTION_NAME}}({{MIN_VALUE}})).toBe({{MIN_EXPECTED}});
  });

  it("should handle boundary max", () => {
    expect({{FUNCTION_NAME}}({{MAX_VALUE}})).toBe({{MAX_EXPECTED}});
  });

  it.each([
    { input: {{NOMINAL_VALUE}}, expected: {{NOMINAL_EXPECTED}} },
    { input: {{ZERO_VALUE}}, expected: {{ZERO_EXPECTED}} },
    // FILL_IN: add equivalence partitioning cases
  ])("should return $expected for input $input", ({ input, expected }) => {
    expect({{FUNCTION_NAME}}(input)).toBe(expected);
  });
});
