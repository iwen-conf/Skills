# Tiered JSON Schema Examples (arc:cartography)

## 1) Canonical Schema

- Base schema file: `../schemas/codemap.schema.json`
- Schema version: `1.0.0`
- Supported tiers: `1 | 2 | 3`

## 2) Common Root Fields

All tiers share the same top-level fields:

```json
{
  "version": "1.0.0",
  "tier": 1,
  "generated_at": "2026-03-06T08:00:00Z",
  "producer_skill": "arc:cartography",
  "path": "/repo",
  "total_tokens_estimate": 1234,
  "entries": []
}
```

## 3) Tier 1 Example (Index)

Use case: fastest repo lookup and initial entry-point scanning.

```json
{
  "version": "1.0.0",
  "tier": 1,
  "generated_at": "2026-03-06T08:00:00Z",
  "producer_skill": "arc:cartography",
  "path": "/repo",
  "total_tokens_estimate": 520,
  "entries": [
    {
      "name": "handler.ts",
      "type": "file",
      "path": "src/api/handler.ts",
      "line_start": 1,
      "language": "typescript",
      "signature": "export async function handleRequest(req: Request)"
    }
  ]
}
```

Expected entry shape (minimum):
- `name`
- `type`
- `path`
- `line_start` (recommended)
- `signature` (if language supports it)

## 4) Tier 2 Example (Context)

Use case: module relationship analysis and implementation planning.

```json
{
  "version": "1.0.0",
  "tier": 2,
  "generated_at": "2026-03-06T08:00:00Z",
  "producer_skill": "arc:cartography",
  "path": "/repo",
  "total_tokens_estimate": 2100,
  "entries": [
    {
      "name": "service.py",
      "type": "file",
      "path": "app/orders/service.py",
      "line_start": 1,
      "language": "python",
      "signature": "class OrderService:",
      "summary": "Coordinates order creation, pricing validation, and persistence.",
      "dependencies": [
        "app.orders.repo.OrderRepository",
        "app.pricing.engine.PricingEngine"
      ]
    }
  ]
}
```

Tier 2 adds:
- `summary`
- `dependencies`

## 5) Tier 3 Example (Full Context)

Use case: deep implementation review, code-level reasoning, and refactor planning.

```json
{
  "version": "1.0.0",
  "tier": 3,
  "generated_at": "2026-03-06T08:00:00Z",
  "producer_skill": "arc:cartography",
  "path": "/repo",
  "total_tokens_estimate": 6900,
  "entries": [
    {
      "name": "auth.go",
      "type": "file",
      "path": "internal/auth/auth.go",
      "line_start": 1,
      "language": "go",
      "signature": "func VerifyToken(ctx context.Context, token string) (*Claims, error)",
      "summary": "Validates JWT and applies tenant boundary checks.",
      "dependencies": [
        "github.com/golang-jwt/jwt/v5",
        "internal/store/tenant"
      ],
      "docstring": "VerifyToken validates token signature and issuer constraints.",
      "code": "package auth\n\nfunc VerifyToken(...) { /* truncated */ }"
    }
  ]
}
```

Tier 3 adds:
- `docstring`
- `code` (possibly truncated)

## 6) Validation Tips

1. Export a tier file:

```bash
python3 arc:cartography/scripts/cartographer.py export --root <project_path> --tier 2 --output codemap.tier2.json
```

2. Validate against schema (example with `ajv`):

```bash
ajv validate -s schemas/codemap.schema.json -d codemap.tier2.json
```
