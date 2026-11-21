# ADR 004: Exit codes follow ESLint

Date: 2026-11
Status: Accepted

## Context

fldlint is a linter and migration tool whose primary surfaces are an ESLint plugin and an `npx` codemod, used in CI. flaglint's contract (flaglint-js ADR 010, flaglint-go ADR 003) has inventory commands that never exit 1 for findings, reserves 1 for `validate`, and uses 3 for internal errors.

## Decision

Use the ESLint convention: `0` clean, `1` problems found, `2` usage or configuration error (including a missing explicit `--config`), `130` SIGINT. Internal errors also exit 2. Details in `spec/contracts/exit-codes.md`. This deliberately differs from flaglint.

## Consequences

- Familiar to JS developers and works with existing CI conventions for linters.
- A finding fails the run by default; `--max-warnings` and baselines manage adoption.
- Scripts written for flaglint cannot assume its codes (notably 3) apply here.
- There is no distinct code for internal errors; stderr carries the detail.
