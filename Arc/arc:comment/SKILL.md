---
name: arc:comment
version: 1.0.0
description: >
  Applies Chinese comment conventions for controllers, contracts, functions,
  structs, fields, and numbered in-function steps. Use when the user says 加注释,
  写注释, 步骤注释, 函数注释, comment style, or documentation comments in code.
  Not for user-facing prose docs.
---

# Code Comment Conventions

## Overview

Use this skill when writing or reviewing code comments for controllers, interfaces, ordinary functions, structs, fields, and numbered implementation-step comments inside functions.

Prefer comments that explain intent, contract, parameters, return values, errors, and operational constraints. Do not add decorative comments or duplicate obvious code.

## When to Use

- Creating or updating controllers, service APIs, repository APIs, domain interfaces, ordinary functions, methods, structs, or DTOs.
- Reviewing comment style consistency.
- A function has meaningful sequential steps that should be documented inline.
- Skip long templates for trivial private helpers when the name already makes behavior unambiguous.
- Not for README, release notes, or user-facing prose (`write` skill).

## Intent Router

| When | Load |
|---|---|
| Numbered steps inside a function body | [`modules/in-function.md`](modules/in-function.md) |
| Interface / usecase Contract / repository port | [`modules/interface.md`](modules/interface.md) |
| Ordinary function or method | [`modules/function.md`](modules/function.md) |
| Struct type or field comments | [`modules/struct.md`](modules/struct.md) |
| HTTP controller / handler | [`modules/controller.md`](modules/controller.md) |

Load only the matching module. Do not load controller templates for a private helper.

## Red Lines

```text
NO DECORATIVE COMMENTS.
NO OBVIOUS SIGNATURE BOILERPLATE ON USECASE CONTRACT METHODS.
NO STALE CALLEE NAMES IN STEP COMMENTS.
NO COMMENTS TO COMPENSATE FOR VAGUE FIELD NAMES — RENAME WITH Repo SUFFIX.
NO EMPTY-STATE DOCUMENTED AS 错误 FOR LIST/SEARCH SUCCESS.
```

## Review Checklist

- Comment templates match the role: controller, interface/contract, ordinary function, struct/field, or in-function steps.
- Function names, route paths, parameter names, return types, and callee names are exact.
- Usecase `Contract` method comments are one-line summaries without repeated `参数` / `返回体` blocks for `ctx`, `param`, `result`, or `err`.
- Private dependency fields are not mechanically annotated; names carry the role, such as `novelCommentRepo repositories.NovelComment`.
- Successful empty/no-data responses are documented as normal returns, not as `错误`, unless the operation is a single-resource lookup where missing data is intentionally `not found`.
- Numbered step comments are continuous and describe blocks rather than single obvious statements.
