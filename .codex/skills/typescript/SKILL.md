---
name: typescript
description: Senior TypeScript development guidance for designing, reviewing, refactoring, and debugging typed JavaScript systems. Use when working with .ts or .tsx files, API and DTO types, generics, narrowing, discriminated unions, strictness errors, type-safe refactors, or replacing unsafe any/unknown usage.
---

# TypeScript

## Workflow

Read the existing tsconfig, lint rules, generated types, API clients, and local naming conventions before changing types. Match the repository's strictness level while improving safety in the touched area.

Use types to encode real contracts at module boundaries. Keep internal implementation types as simple as possible and avoid clever type machinery unless it prevents a concrete class of bugs.

## Type Design

Prefer precise domain types, discriminated unions, and narrow interfaces for public boundaries. Use type aliases or interfaces consistently with the surrounding code.

Model nullable, optional, loading, error, and empty states explicitly. Do not erase uncertainty with non-null assertions unless the invariant is already enforced nearby and obvious.

Avoid `any`. Use `unknown` at untrusted boundaries, then narrow with runtime checks, schema validation, or existing parser helpers.

## Generics and Narrowing

Use generics when the caller needs to preserve relationships between inputs and outputs. Avoid generic parameters that only rename `unknown` or make call sites harder to read.

Prefer readable narrowing: discriminant checks, predicate functions, exhaustive switches, and small validation helpers. Add `never` exhaustiveness checks where the codebase already uses them or where future variants are likely.

## API and Refactor Safety

Keep API DTOs separate from view models when transformation adds meaning. Do not let transport quirks leak through the UI unless the project already treats API shapes as domain shapes.

During refactors, change types and call sites together. Let compiler errors guide the migration, then remove temporary compatibility types once the touched surface is consistent.

## Verification

Run the repository's typecheck, tests, and lint commands when available. If a command is expensive or unavailable, state the remaining type-safety risk clearly.
