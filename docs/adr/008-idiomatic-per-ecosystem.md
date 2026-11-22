# ADR 008: Idiomatic per ecosystem

Date: 2026-11
Status: Accepted

## Context

fldlint ships one tool per language (JS/TS, Go, later Java). An earlier draft
gave all of them one process contract: the same exit codes, flags and a
default `--max-warnings 0`. That is unfamiliar to every ecosystem at once. A
Go developer expects `go vet`/`x/tools` behaviour, a JS developer expects
ESLint, a Java developer expects the Maven/Gradle rewrite plugin. The
maintainer decided that nothing should catch a developer off guard.

## Decision

Each language's tool is as idiomatic as possible for its own ecosystem.

Shared across languages (the contract; what a CI system can aggregate):
rule ids and `spec/rules.yaml`, message text, positions (1-based line, UTF-16
column, tab = 1), the fingerprint, the baseline and SARIF shapes, and the
fixtures.

Per ecosystem (not shared): exit codes, flags, severity configuration, config
file format, fix workflow and help text.

- JS/TS follows ESLint exactly: exit `0`/`1`/`2`, `--max-warnings` default `-1`
  (unlimited; the earlier default of 0 is withdrawn), per-rule `off`/`warn`/`error`
  in flat config, `recommended` sets every rule to `warn`.
- Go follows `go/analysis` (`singlechecker`/`unitchecker`, `go vet -vettool`,
  `go fix -fixtool`). No severity or `--max-warnings`; golangci-lint, gopls or
  `go vet` decide what fails.
- Java follows the Maven/Gradle rewrite plugin conventions (M3, unspecified).

Details: [exit-codes.md](../../spec/contracts/exit-codes.md).

## Consequences

- Developers get the behaviour they already know; docs can point at ESLint or
  `go vet` for process questions.
- No cross-language exit-code or flag contract; CI that aggregates languages
  must rely only on the shared items, typically SARIF.
- A finding's severity is configuration, not data: `rules.yaml` severities are
  only the `recommended` defaults.
- Contributors must not port one ecosystem's conventions into another
  (see CONTRIBUTING.md, Principles).
