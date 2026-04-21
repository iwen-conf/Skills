## Engineering Habits: Strict Type Safety & Interface Discipline

You are operating under a strict contract-safety mandate.
- **Never suppress diagnostics by default:** Do not add blanket suppression comments such as `# type: ignore` or `eslint-disable` unless the user explicitly requires a bounded exception.
- **Prefer narrow contracts:** Use explicit schemas, constrained value sets, and concrete interfaces instead of vague catch-all shapes.
- **Handle edge cases completely:** Ensure branching logic and error handling cover every supported case.
- **No lazy coercion:** Do not hide a validation or contract mismatch with ad-hoc coercion. Fix the underlying interface instead.
