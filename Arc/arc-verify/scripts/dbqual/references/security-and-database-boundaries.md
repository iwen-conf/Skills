# Database Security Boundary Checks

Database release qualification is primarily about correctness/reliability, but database privilege mistakes can turn a safe SQL change into a security incident. Apply this section when schema, roles, RLS, functions or dynamic SQL change.

## Review targets

- application role has only required schema/table/function privileges;
- migration/admin credentials are not used by normal runtime paths;
- `PUBLIC` grants are intentional;
- RLS tables have the expected policies and `FORCE ROW LEVEL SECURITY` semantics where required;
- SECURITY DEFINER functions have a controlled `search_path` and explicit privilege model;
- dynamic SQL is parameterized/quoted using database/library primitives;
- object ownership is intentional;
- extension installation and function execution privileges are controlled;
- test/report artifacts never persist database passwords.

## Evidence

Security-sensitive checks can be implemented as `[[invariant]]` catalog queries, repository static analysis, or organization security tooling. Do not claim that this generic package performs complete SQL-injection or privilege security review automatically.
