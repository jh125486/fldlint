# ADR 002: Share the spec as data, not an engine

Date: 2026-11
Status: Accepted

## Context

A migration tool must parse and type-check each language: TypeScript needs the TS compiler/typescript-eslint, Go needs `go/types` and `x/tools`, Java needs OpenRewrite. Safe rewriting depends on scope, type and binding analysis that has no language-neutral form. Options: one engine (for example WASM or a service) used from every language, or separate engines.

## Decision

Each language has its own detection engine on its native parser and type system. What is shared is data: `rules.yaml`, JSON Schemas, fixtures (input and expected output per language) and the contracts in `spec/contracts/`. Each language generates a typed table from `rules.yaml` (committed, drift checked in CI) and a conformance harness runs the shared fixtures against its engine. A single planner function per language returns rewrite or skip.

## Consequences

- Idiomatic tooling per ecosystem: an ESLint plugin, a codemod, a `go/analysis` analyzer, an OpenRewrite recipe.
- Behavior stays aligned through fixtures and contracts, not through shared code.
- Detection logic is implemented once per language; that cost is accepted.
- Rule coverage differs per language only where `rules.yaml` says so.
