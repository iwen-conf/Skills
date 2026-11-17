import { isJSONObject, JSONNumberLiteral } from "./json-values";

export function sameLineageJSON(left: unknown, right: unknown): boolean {
  if (left === right) return true;
  if (left instanceof JSONNumberLiteral || right instanceof JSONNumberLiteral)
    return (
      left instanceof JSONNumberLiteral &&
      right instanceof JSONNumberLiteral &&
      left.raw === right.raw
    );
  if (Array.isArray(left) || Array.isArray(right))
    return (
      Array.isArray(left) &&
      Array.isArray(right) &&
      left.length === right.length &&
      left.every((value, index) => sameLineageJSON(value, right[index]))
    );
  if (!isJSONObject(left) || !isJSONObject(right)) return false;
  const keys = Object.keys(left);
  return (
    keys.length === Object.keys(right).length &&
    keys.every((key) => Object.hasOwn(right, key) && sameLineageJSON(left[key], right[key]))
  );
}

/** Structural limits mirror the Go contract reader; no predicate is evaluated here. */
export function lineagePredicateReferences(
  value: unknown,
  bad: (message: string) => never,
): Set<string> {
  const references = new Set<string>();
  let count = 0;
  const operand = (value: unknown) => {
    if (!isJSONObject(value) || Boolean(value.ref) === Boolean(value.literal))
      bad("谓词操作数必须且只能包含 ref 或 literal");
    const item = value as Record<string, unknown>;
    if (item.ref) {
      if (
        !isJSONObject(item.ref) ||
        typeof item.ref.subject !== "string" ||
        !item.ref.subject ||
        typeof item.ref.attribute !== "string" ||
        !item.ref.attribute
      )
        bad("谓词属性引用缺少主体或属性");
      const ref = item.ref as Record<string, string>;
      references.add(`${ref.subject}.${ref.attribute}`);
    } else if (
      !isJSONObject(item.literal) ||
      typeof item.literal.type !== "string" ||
      !item.literal.type ||
      !Object.hasOwn(item.literal, "value")
    )
      bad("谓词字面量缺少类型或值");
  };
  const walk = (value: unknown, depth: number) => {
    if (!isJSONObject(value) || depth > 32 || ++count > 2048)
      bad("谓词结构非法或超过深度 32 / 节点 2048");
    const predicate = value as Record<string, unknown>;
    const op = predicate.op;
    if (
      typeof op !== "string" ||
      ![
        "true",
        "false",
        "and",
        "or",
        "not",
        "eq",
        "ne",
        "lt",
        "lte",
        "gt",
        "gte",
        "in",
        "contains",
        "subset",
        "intersects",
        "between",
      ].includes(op)
    )
      bad("谓词运算符不受支持");
    if (predicate.args !== undefined && !Array.isArray(predicate.args)) bad("谓词 args 不是数组");
    const args = (predicate.args ?? []) as unknown[];
    if (["and", "or", "not"].includes(String(op))) {
      if (
        !args.length ||
        (op === "not" && args.length !== 1) ||
        predicate.left !== undefined ||
        predicate.right !== undefined
      )
        bad("逻辑谓词的子项或操作数非法");
      args.forEach((child) => walk(child, depth + 1));
    } else if (op === "true" || op === "false") {
      if (args.length || predicate.left !== undefined || predicate.right !== undefined)
        bad("常量谓词不能包含操作数");
    } else {
      if (args.length) bad("属性比较不能包含逻辑子项");
      operand(predicate.left);
      operand(predicate.right);
    }
  };
  walk(value, 0);
  return references;
}
