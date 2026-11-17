import { describe, expect, it } from "vite-plus/test";
import { displayValue } from "./contract";
import { isJSONObject, JSONNumberLiteral, parseSnapshotJSON } from "./json-values";
import { readSnapshotText } from "./snapshot";
import { contractFixture } from "./test-fixtures";

describe("source numeric tokens in displayed evidence", () => {
  it("preserves i64 limits, unsafe integers, fractional/exponent spelling and negative zero", () => {
    const values = parseSnapshotJSON(
      "[9223372036854775807,-9223372036854775808,9007199254740993,100.0,1e2,-0,0.1000000000000000001]",
    ) as unknown[];
    expect(values.every((value) => value instanceof JSONNumberLiteral)).toBe(true);
    expect(values.map(displayValue)).toEqual([
      "9223372036854775807",
      "-9223372036854775808",
      "9007199254740993",
      "100.0",
      "1e2",
      "-0",
      "0.1000000000000000001",
    ]);
    expect(parseSnapshotJSON("[0,100,1.25,true,null]")).toEqual([0, 100, 1.25, true, null]);
  });
  it("keeps JSON strings distinguishable and does not interpret object contents as numeric markers", () => {
    const value = parseSnapshotJSON(
      '{"string":"9007199254740993","raw":"1e2","nested":[100.0,"100.0"],"escaped":"a\\\"b\\\\c","__proto__":{"safe":true}}',
    ) as Record<string, unknown>;
    expect(displayValue(value.string)).toBe('"9007199254740993"');
    expect(displayValue(value.nested)).toBe('[100.0, "100.0"]');
    expect(value.escaped).toBe('a"b\\c');
    expect(Object.getPrototypeOf(value)).toBe(Object.prototype);
    expect(Object.hasOwn(value, "__proto__")).toBe(true);
    expect(displayValue({ raw: "1e2" })).toBe('{"raw": "1e2"}');
  });
  it("rejects malformed JSON and ambiguous duplicate keys including escaped names", () => {
    for (const value of [
      '{"a":01}',
      "[1,]",
      '{"a":+1}',
      '{"a":1} trailing',
      '{"a":9007199254740993,"a":1}',
      '{"a":1,"\\u0061":2}',
    ])
      expect(() => parseSnapshotJSON(value)).toThrow();
  });
  it("preserves Unicode, escaped quotes and nested numeric-looking strings", () => {
    const source =
      '{"\\u4e3b\\u4f53":{"label":"订单123 / \\ud83d\\ude00","nested":[{"text":"1e2","number":1e2}],"quote":"a\\\"b"}}';
    const value = parseSnapshotJSON(source) as {
      主体: { label: string; nested: { text: string; number: unknown }[]; quote: string };
    };
    expect(value.主体.label).toBe("订单123 / 😀");
    expect(value.主体.nested[0].text).toBe("1e2");
    expect(displayValue(value.主体.nested[0].number)).toBe("1e2");
    expect(value.主体.quote).toBe('a"b');
  });
  it("preserves the actual token through the production snapshot text reader without recomputing verdicts", () => {
    const fixture = contractFixture();
    fixture.runs[0].facts![0].requests = [
      {
        actor: "merchant",
        action_index: 0,
        node: "create",
        template_sha256: "synthetic",
        request_sha256: "synthetic",
        values: { "request.amount": "__raw_numeric_test__" },
      },
    ];
    const source = JSON.stringify(fixture).replace('"__raw_numeric_test__"', "9223372036854775807");
    const snapshot = readSnapshotText(source);
    expect(displayValue(snapshot.runs[0].facts![0].requests![0].values?.["request.amount"])).toBe(
      "9223372036854775807",
    );
    expect(snapshot.runs[0].facts![0].verdict).toBe(fixture.runs[0].facts![0].verdict);
  });
  it("never treats an exact numeric token as a source JSON object", () => {
    expect(isJSONObject(parseSnapshotJSON("9223372036854775807"))).toBe(false);
    expect(isJSONObject(parseSnapshotJSON('{"raw":"9223372036854775807"}'))).toBe(true);
    const fixture = contractFixture();
    const text = JSON.stringify(fixture).replace(
      /"values":\{[^}]+\}/,
      '"values":9223372036854775807',
    );
    expect(() => readSnapshotText(text)).toThrow();
  });
});
