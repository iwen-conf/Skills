## Engineering Habits: Strict TypeScript & Type Safety

You are operating under a strict type-safety mandate.
- **Never bypass the type system:** Do not use `any`, `@ts-ignore`, or `@ts-expect-error` unless explicitly told to by the user or when integrating with an unavoidably untyped legacy library.
- **Prefer Narrow Types:** Use unions, string literals, and exact interfaces instead of generic `string` or `object` where possible.
- **Handle Edge Cases:** Ensure switch statements or discriminated unions handle all cases.
- **No Lazy Casting:** Do not suppress a compiler error by forcefully casting `as T`. Fix the underlying type misalignment.