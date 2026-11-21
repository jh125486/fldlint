# ADR 004: Exit codes follow ESLint

Date: 2026-11
Status: Accepted
Amended 2026-11: narrowed to the JS/TS tools by [ADR 008](008-idiomatic-per-ecosystem.md); the `--max-warnings` default is `-1` (ESLint's), not 0; `130` is not part of the contract.

## Context

fldlint is a linter and migration tool whose primary surfaces are an ESLint plugin and an `npx` codemod, used in CI. flaglint's contract (flaglint-js ADR 010, flaglint-go ADR 003) has inventory commands that never exit 1 for findings, reserves 1 for `validate`, and uses 3 for internal errors.

## Decision

For the JS/TS tools (ESLint plugin and CLI) use the ESLint convention: `0` no errors and warnings within `--max-warnings`, `1` errors or too many warnings, `2` usage, configuration or internal error. Go and Java follow their own ecosystems (ADR 008). Details in `spec/contracts/exit-codes.md`. This deliberately differs from flaglint.

## Consequences

- Familiar to JS developers and works with existing CI conventions for linters.
- Severity is configured per rule; teams gate CI with `error` severity or `--max-warnings 0`.
- Scripts written for flaglint cannot assume its codes (notably 3) apply here.
- There is no distinct code for internal errors; stderr carries the detail.
