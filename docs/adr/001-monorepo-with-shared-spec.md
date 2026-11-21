# ADR 001: Monorepo with a shared spec

Date: 2026-11
Status: Accepted
Amended 2026-11: the "ESLint-style exit codes" scope note was replaced; process behaviour is per ecosystem ([ADR 008](008-idiomatic-per-ecosystem.md)).

## Context

fldlint migrates LaunchDarkly SDK usage to OpenFeature in JS/TS, Go and later Java. Existing tools (flaglint-js, flaglint-go) are separate repositories with separate maintainers, kept aligned by a prose cross-tool contract that drifts. We could contribute upstream or start our own repository.

## Decision

Create one new monorepo, `fldlint`, with a language-neutral `spec/` (rules, schemas, fixtures, contracts) and one directory per language (`js/`, `go/`, `java/`). Do not contribute upstream: the scope (a rewrite-safety model with per-ecosystem process behaviour (ADR 008), a different fingerprint) differs from upstream's inventory-oriented design, and a single repo lets one change update the spec and every language's conformance run atomically.

## Consequences

- Spec and implementations change in one PR; CI can fail on any drift.
- Each language still builds and tests independently (`npm test`, `go test`).
- We take on maintenance of a separate project, and must keep attribution to upstream clear (ADR 005).
- The name is similar to flaglint's; the README says plainly that they are different projects.
