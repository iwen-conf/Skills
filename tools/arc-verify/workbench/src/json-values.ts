/** A display-only JSON numeric token. It is never used to evaluate business predicates. */
export class JSONNumberLiteral {
  readonly raw: string;
  constructor(raw: string) {
    this.raw = raw;
    Object.freeze(this);
  }
}

export function isJSONObject(value: unknown): value is Record<string, unknown> {
  return (
    value !== null &&
    typeof value === "object" &&
    !Array.isArray(value) &&
    !(value instanceof JSONNumberLiteral)
  );
}

/** Keep source numeric spelling where JavaScript would round or normalize it. */
export function parseSnapshotJSON(text: string): unknown {
  // Native parsing validates the complete JSON grammar before the token walk.
  JSON.parse(text);
  let cursor = 0;
  const number = /-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/y;
  const whitespace = () => {
    while (/\s/.test(text[cursor] ?? "") && cursor < text.length) cursor++;
  };
  const string = (): string => {
    const start = cursor++;
    while (cursor < text.length) {
      if (text[cursor] === "\\") cursor += 2;
      else if (text[cursor++] === '"') return JSON.parse(text.slice(start, cursor));
    }
    throw new SyntaxError("Unterminated JSON string");
  };
  const read = (depth: number): unknown => {
    if (depth > 256) throw new SyntaxError("Snapshot JSON nesting exceeds 256");
    whitespace();
    const char = text[cursor];
    if (char === '"') return string();
    if (char === "{") {
      const object: Record<string, unknown> = {};
      cursor++;
      whitespace();
      while (text[cursor] !== "}") {
        const key = string();
        if (Object.hasOwn(object, key))
          throw new SyntaxError(`Duplicate JSON key ${JSON.stringify(key)}`);
        whitespace();
        cursor++; // colon; the native parser has already validated it
        const value = read(depth + 1);
        Object.defineProperty(object, key, {
          value,
          enumerable: true,
          configurable: true,
          writable: true,
        });
        whitespace();
        if (text[cursor] !== ",") break;
        cursor++;
        whitespace();
      }
      cursor++;
      return object;
    }
    if (char === "[") {
      const array: unknown[] = [];
      cursor++;
      whitespace();
      while (text[cursor] !== "]") {
        array.push(read(depth + 1));
        whitespace();
        if (text[cursor] !== ",") break;
        cursor++;
      }
      cursor++;
      return array;
    }
    for (const [literal, value] of [
      ["true", true],
      ["false", false],
      ["null", null],
    ] as const) {
      if (text.startsWith(literal, cursor)) {
        cursor += literal.length;
        return value;
      }
    }
    number.lastIndex = cursor;
    const raw = number.exec(text)?.[0];
    if (raw === undefined) throw new SyntaxError(`Invalid JSON value at ${cursor}`);
    cursor += raw.length;
    const parsed = Number(raw);
    return (Number.isInteger(parsed) && !Number.isSafeInteger(parsed)) ||
      JSON.stringify(parsed) !== raw
      ? new JSONNumberLiteral(raw)
      : parsed;
  };
  return read(0);
}

export function jsonValueText(value: unknown): string {
  if (value instanceof JSONNumberLiteral) return value.raw;
  if (Array.isArray(value)) return `[${value.map(jsonValueText).join(", ")}]`;
  if (value !== null && typeof value === "object")
    return `{${Object.entries(value)
      .map(([key, item]) => `${JSON.stringify(key)}: ${jsonValueText(item)}`)
      .join(", ")}}`;
  return JSON.stringify(value) ?? "未观察";
}
