import { describe, it, expect, beforeAll, afterAll } from "vitest";
import { {{FUNCTION_NAME}} } from "{{MODULE_PATH}}";

describe("{{FUNCTION_NAME}} — integration", () => {
  // Shared state for cross-module dependencies
  let connection;

  beforeAll(async () => {
    // FILL_IN: setup — initialize DB, HTTP client, service connections
    connection = undefined;
  });

  afterAll(async () => {
    // FILL_IN: teardown — close connections, clean state
    connection = undefined;
  });

  it("should work across module boundaries with nominal input", () => {
    expect({{FUNCTION_NAME}}({{SAMPLE_INPUT}})).toBe({{EXPECTED_OUTPUT}});
  });

  it("should propagate errors from dependencies correctly", () => {
    expect(() => {{FUNCTION_NAME}}({{INVALID_INPUT}})).toThrow();
  });

  it("should maintain data integrity through processing layers", () => {
    const result = {{FUNCTION_NAME}}({{NOMINAL_VALUE}});
    expect(result).toBe({{NOMINAL_EXPECTED}});
  });
});
