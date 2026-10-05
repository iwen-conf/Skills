# Database security boundary standard

Database release qualification is incomplete when a schema/function change changes authorization semantics.

Review when applicable:

```text
application role privileges
owner vs runtime role separation
SUPERUSER/CREATEROLE/CREATEDB grants
PUBLIC grants
row-level security and FORCE RLS expectations
SECURITY DEFINER functions and controlled search_path
untrusted or unexpected extensions
schema ownership and CREATE rights
cross-tenant predicates and indexes
migration role privileges
replication/subscription role privileges
```

Do not impose RLS or a privilege model universally. The contract must come from the application's security design. The audit script supports explicit role/RLS/extension policies and custom read-only assertions.
