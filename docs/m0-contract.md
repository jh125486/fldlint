# M0 contract (frozen)

This file is the shared contract for the M0 scaffold work packages (WP1-WP6).
Names and paths here are fixed. If something is missing or wrong, do not
invent a different convention: stop and report it back to the orchestrator.

## Project

`fldlint` lints and migrates LaunchDarkly SDK usage to OpenFeature in JS/TS,
Go and (later) Java. One monorepo, one language-neutral spec as the source of
truth. Each language has its own detection engine; they share DATA (rules,
fixtures, contracts), never an engine.

## Layout and ownership

```
spec/            rules.yaml, schema/, fixtures/, contracts/        (WP1, WP2, WP3)
js/              npm workspaces: packages/{spec,eslint-plugin,codemod,cli}  (WP4)
go/              go.mod, internal/spec, conformance, analyzer stub (WP5)
java/            README.md only in M0                              (WP6)
docs/adr/        ADRs                                              (WP3)
.github/ Makefile .gitignore .editorconfig CODEOWNERS dependabot  (WP6)
LICENSE NOTICE README.md CONTRIBUTING.md                           (WP3)
docs/m0-contract.md                                                (orchestrator; read-only for everyone else)
```

A work package may only create or modify files it owns. Do not touch files
owned by another package, even to fix a typo; report it instead.

## Rules file: `spec/rules.yaml`

```yaml
version: 1
skipCategories:
  detail:           {severity: warning, message: "Detail call; reason/variation metadata has no 1:1 rewrite"}
  bulk:             {severity: warning, message: "Bulk evaluation; migrate manually"}
  dynamic-key:      {severity: warning, message: "Flag key is not a static string"}
  unbound-client:   {severity: warning, message: "OpenFeature client binding not proven in scope"}
  type-mismatch:    {severity: warning, message: "Return/default type differs between SDKs"}
  context-unproven: {severity: warning, message: "Cannot prove LD context -> EvaluationContext conversion"}
rules:
  - id: bool-variation
    valueType: boolean          # boolean | string | number | integer | object
    risk: low                   # low | medium | high
    keyArg: 0
    defaultArg: 2
    contextArg: 1
    languages:
      js:   {methods: [boolVariation], async: true,
             template: "{{client}}.getBooleanValue({{key}}, {{default}}, {{context}})"}
      go:   {methods: [BoolVariation, BoolVariationCtx], ctxArg: {BoolVariationCtx: 0},
             template: "{{client}}.BooleanValue({{goctx}}, {{key}}, {{default}}, {{evalctx}})",
             requires: [context-unproven]}
      java: {methods: [boolVariation],
             template: "#{any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key}, #{default}, #{context})"}
  - id: bool-variation-detail
    risk: high
    languages:
      js: {methods: [boolVariationDetail]}
      go: {methods: [BoolVariationDetail]}
    skip: detail                # report-only; a rule has `skip` XOR per-language `template`
```

- Placeholders are an enum: `client key default context goctx evalctx`.
- Schemas: `spec/schema/rules.schema.json` and `spec/schema/fixture.schema.json`
  (JSON Schema draft 2020-12).
- `rules.yaml` must cover every method of the LaunchDarkly Node server SDK and
  go-server-sdk v7 listed by flaglint (Variation/Detail/Ctx/bulk). Value-type
  mismatches (Go `IntVariation` -> int64, `JSONVariation` -> any) use
  `skip: type-mismatch`.

## Fixtures: `spec/fixtures/<rule-id>/<case-id>/`

```
case.yaml
js/input.ts        js/output.ts           (output omitted when nothing is rewritten)
go/input.go        go/output.go.golden
java/Before.java   java/After.java        (M3; absent means not applicable)
```

`case.yaml`:

```yaml
id: static-key-double-quote
rule: bool-variation
tags: [quote-double]            # see mandatory tags below
description: one sentence
expect:
  js: {diagnostics: [{line: 3, col: 18, ruleId: bool-variation}], output: true}
  go: {diagnostics: [{line: 9, col: 12, ruleId: bool-variation, skip: context-unproven}], output: false}
```

- `diagnostics[].line/col` are 1-based. `skip` is a key of `skipCategories`
  and means "reported, not rewritten". `output: true` means the language
  directory contains the expected rewritten file.
- A missing language directory/`expect` entry means "not applicable".
- Mandatory shape tags every rewriting rule must cover at least once:
  `quote-single`, `quote-double`, `typed-decl`, `multi-declarator`,
  `shadowed-client`, `dynamic-key`, `aliased-import`.

## Planner contract (all languages)

One function per language takes a detected call site and returns a result:
`{kind: "rewrite", edit}` or `{kind: "skip", category, ruleId}`. The scanner,
ESLint rule, codemod and analyzer all call it; none re-implement detection.
In M0 only the types and a stub implementation exist.

## Codegen

Each language generates a typed table from `spec/rules.yaml`. Generated files
are committed and CI fails on any `git diff` after regenerating.

- Go: `go/internal/spec/rules_gen.go` via `go run ./internal/spec/gen` (run from `go/`).
- JS: `js/packages/spec/src/rules.gen.ts` via `npm run gen -w @fldlint/spec` (run from `js/`).

## Make targets (the ONLY entry points CI calls; owned by WP6)

`make spec-check` (schema-validate rules.yaml + every case.yaml, run both
codegens, fail on drift, fixture tag-coverage check), `make spec-gen`,
`make js-test`, `make go-test`, `make lint`, `make test` (all).
WP4 and WP5 must make `npm test` (in `js/`) and `go test ./...` (in `go/`) work
standalone; WP6's targets wrap them. WP4 exposes `npm run spec:validate` (ajv
over rules.yaml and all case.yaml); `make spec-check` calls it.

## Conformance harness behavior in M0

No real detection exists yet. Every fixture case is discovered and counted, and
must be reported as pending (skipped, not failed) per language until that
language's engine lands. The harness must be real: it must load cases,
validate them against the schema, and fail on a malformed case.

## Exit codes (output of the CLIs, documented by WP3)

`0` clean, `1` problems found, `2` usage or config error (including a missing
explicit `--config` path), `130` SIGINT. Results to stdout, diagnostics to
stderr, line format `file:line:col  rule-id  message`. JSON and SARIF
formatters. `--max-warnings` and `--quiet`. No `v2` labelling anywhere.

## Fingerprint

A clean new format (not flaglint's), designed and documented by WP3 in
`spec/contracts/fingerprint.md`. Must be deterministic, path-normalized
(`/`, no leading `./`), independent of language, and stable under unrelated
edits to the file.

## Approved dependencies

- JS: `eslint` (9.x), `@typescript-eslint/parser`, `@typescript-eslint/utils`,
  `ajv` (+`ajv-formats` if needed), `yaml`, `vitest`, `typescript`.
- Go: `golang.org/x/tools`, `gopkg.in/yaml.v3`. Go tests use stdlib `testing`
  plus `testing/synctest` where timing matters; `testify` is NOT approved.
- Anything else: stop and ask.

## Go test conventions (mandatory for all Go tests)

Table-driven tests; `t.Parallel()` in tests and subtests; blackbox tests in
`package foo_test` unless internals are needed (then `package foo`, file named
`*_internal_test.go`); one test function per exported function; use
`t.Context()` not `context.Background()`; use `testing/synctest` for timing.
Go version: use the toolchain installed locally; set `go.mod` to the lowest
version that works.

## Attribution

fldlint may port targeted algorithms from MIT-licensed flaglint/flaglint-js and
flaglint/flaglint-go. Every ported file keeps an SPDX header and a
`Derived from flaglint-<x> <path> @<sha>` comment. `NOTICE` (WP3) lists the
upstream repos and SHAs. Do not copy whole modules.

## Working rules for every work package

- Work only in your assigned git worktree on your assigned branch.
- Commit with `git commit -s` using conventional-commit messages and end the
  message with the line: `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`
- Do not push, open PRs, or touch other branches. The orchestrator merges.
- Never commit secrets. Do not edit `vendor/`. Do not add unapproved deps.
- Run your own verification (tests/lint) before finishing and report the
  commands and results honestly, including failures and anything skipped.
