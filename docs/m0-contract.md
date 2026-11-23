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
  default-type-unproven: {severity: warning, message: "Default value type not proven"}
  receiver-unproven: {severity: warning, message: "LaunchDarkly client expression cannot be removed safely"}
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

Exception: CI runs `golangci-lint` through `golangci/golangci-lint-action` pinned to
the same version as local `make lint`; every other CI step calls a Make target.

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

SUPERSEDED by M0.3 (idiomatic per ecosystem): exit codes, flags and severity
behaviour are per ecosystem, not shared. See spec/contracts/exit-codes.md. Still
true everywhere: results to stdout, diagnostics to stderr, line format
`file:line:col  rule-id  message`, JSON and SARIF formatters, and no `v2`
labelling anywhere.

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

## M0.1 amendments (post-review; these override anything above that conflicts)

Decided after the first integration review. Authors working on the fix round
must follow these.

### Toolchain
- SUPERSEDED by M0.6 (Node/TypeScript/@types/node lines): Node `>=22.12`; CI matrix `[22, 24]`; `@types/node ^22`.
- `vitest ^5.1.1` (approved) still stands.
- Node 20 is dropped (end of life).

### Context policy (both languages)
A rewrite that changes the evaluation-context argument is only allowed when
the LaunchDarkly context is a simple single-kind literal. Otherwise the call is
reported as `skip: context-unproven`. So JS rules also carry
`requires: [context-unproven]`, same as Go.
- JS: rewrite only when the context argument is an object literal whose only
  properties are `key` (any expression) and optionally `kind: "user"`; emit
  `{ targetingKey: <key expr> }`. Any other context expression (identifier,
  spread, extra attributes, other kinds, multi-kind) is skipped.
- Go: rewrite only for `ldcontext.New(<expr>)`; emit
  `openfeature.NewEvaluationContext(<expr>, nil)`. Anything else is skipped.
- The JS `complex-context` fixture therefore becomes a skip, like Go's.

### Placeholder semantics (document in spec/contracts/placeholders.md)
- `{{client}}`: identifier of the proven OpenFeature client binding in scope.
- `{{key}}`, `{{default}}`: source text of those arguments.
- `{{context}}` (JS) / `{{evalctx}}` (Go): the converted context per the policy above.
- `{{goctx}}`: for `*Ctx` variants, the source text of argument 0. For plain
  variants, the enclosing function's `context.Context` parameter name if there
  is exactly one, else `context.TODO()` (add `"context"` to imports if absent).
  If inserting `context.TODO()` and the identifier `context` is shadowed in
  scope at the call, skip with `context-unproven`.

### Compilable output
Rewrites must leave code that compiles. Go: remove an import only if the file
no longer references it (e.g. `ldcontext` after its only use is rewritten); a
rewrite must not leave an unused local variable, so Go fixtures keep the LD
client referenced elsewhere (e.g. `defer ldClient.Close()`). TypeScript
tolerates unused locals only by default (it is an error under
`--noUnusedLocals`, TS6133; see the M0.5 amendment), so JS outputs may keep the
now-unused client; state this difference in the contracts.

### Fixture coverage
- The seven mandatory shape tags apply to the reference rule `bool-variation`
  only (detection shapes are shared engine behaviour, not per-method).
- Every other rewriting rule needs at least one rewrite case per language that
  has a template, in a directory named for the rule. (M0.1 originally also
  required a skip case; M0.2 dropped that, and M0.2b limits M0 coverage to js
  and go.)
- Add `*Ctx` Go cases, including a `context`-named-parameter case, and a
  `migrationVariation` skip case.
- JS fixtures import `@launchdarkly/node-server-sdk` (not the legacy
  `launchdarkly-node-server-sdk`, whose typings lack `boolVariation`).
- `spec/rules.yaml`: add a `migration-variation` rule (`skip: type-mismatch`)
  for JS `migrationVariation` and Go `MigrationVariation`/`MigrationVariationCtx`.
  Drop JS `allFlags` and `isFeatureEnabled` only if verified absent from the
  current LD Node SDK typings; otherwise keep them and say where they exist.

### Validator parity (JS and Go must accept and reject the same inputs)
Both loaders: reject unknown keys, enforce enums/patterns/required fields,
integer `line`/`col`, `output` boolean, non-null language entries; reject a
case whose `rule` or any `diagnostics[].ruleId` is not in `rules.yaml`; reject
`ctxArg` for a method not in `methods`; reject the same method appearing in two
rules for one language; reject a case that promises `expect.<lang>` without the
language input file, or `output: true` without the output file (and vice versa).
JS exposes `npm run spec:tags` (fails, naming rule and missing tag, when the
coverage rules above are not met); the Makefile `fixture-tags` target calls it.

### Output contract
Reconcile `--quiet` with severities: `--quiet` suppresses findings below the
configured failing threshold only in output and never changes the exit code;
define a message for rewritable findings (add a `message` field to rewriting
rules in rules.yaml, with placeholders limited to `{{key}}`).

### CI
CodeQL and dependency-review run only when the repo is public
(`github.event.repository.private == false`). Pin the latest golangci-lint v2.x
(same `gh api repos/golangci/golangci-lint/releases/latest` lookup for everyone)
and give `go/` a `.golangci.yml`. Use `BASE...HEAD` for path filters and
`git status --porcelain` for drift checks.

## M0.2 amendments (second review round; override anything above that conflicts)

### Fixture discovery and semantics (JS and Go validators must both enforce)
- Every directory under `spec/fixtures/<rule>/<case>/` must contain `case.yaml`
  (not `case.yml`); any other directory depth, or a directory without
  `case.yaml`, is an error. Case ids are unique across the whole spec.
- A case's `rule` must exist; every `diagnostics[].ruleId` must equal the case's
  `rule`; `diagnostics[].skip`, when present, must equal that rule's own `skip`,
  and a rule with `skip` requires every diagnostic to carry it; a rewriting rule's
  diagnostics carry no `skip` unless the language has no template for it (then the
  language must not appear in `expect` at all).
- `skip` on a diagnostic implies `output: false`; `output: true` requires at
  least one diagnostic, and the output file must differ from the input file.
- `expect.<lang>` is only allowed when the rule binds that language
  (`languages.<lang>` exists).

### Coverage (`spec:tags`)
- `bool-variation` covers the seven shape tags (unchanged).
- Every rewriting rule (no `skip`) needs at least one rewrite case (`output: true`,
  >=1 diagnostic) for EACH language that has a template. The earlier claim of
  "and one skip case" is dropped everywhere.

### Positions
`line`/`col` are 1-based; `col` counts UTF-16 code units from the start of the
line, a tab counts as one column (SARIF's default `columnKind`). Go
implementations must convert from byte offsets. Add a non-ASCII fixture where a
multi-byte character precedes the call on the same line.

### Fix-mode exit code
A run that applies fixes uses the same exit-code rules as a check run, evaluated
on what REMAINS after fixing (like ESLint `--fix`): any remaining non-baselined
finding exits per the ecosystem rules (JS: errors, or warnings over
`--max-warnings`, which defaults to unlimited, exit 1); everything fixed exits 0.
Baselined findings are excluded from `summary.warnings`/`errors` counts and
reported in `summary.baselined`; add an `info` counter.

### Rule messages
Messages are shared across languages, so they must not name a language API
(no `getBooleanValue`/`BooleanValue`); use neutral wording such as
"Migrate boolean flag {{key}} to OpenFeature".

### CI gating
CodeQL and dependency-review are gated by the repository variable
`vars.GHAS_ENABLED == 'true'` (not by repository privacy, which is absent from
`schedule` events). Document setting it when the repo goes public. In the CodeQL
job, `actions/setup-go` runs BEFORE `github/codeql-action/init`.

### Loaders
- An explicit `-spec` (Go) / `--spec-root` (JS) that is missing or invalid is a
  hard error, never a silent fallback to the stub. The stub fallback only applies
  when no spec root is given at all.
- Go message regex rejects a lone `}` like the schema (`[^{}]`).
- Known, deferred to a tracked issue (do NOT work on it now): YAML scalar typing
  differences between ajv's YAML parser and yaml.v3 (octal/binary/underscore/1e17
  integers, timestamps, custom tags, `%YAML 1.1`).

### M0.2a correction (supersedes the M0.2 diagnostic/skip bullets above)

There are two kinds of skip. Do not conflate them:
- **Rule-level skip** (`skip:` on the rule in rules.yaml: `detail`, `bulk`,
  `type-mismatch`): the call shape is never rewritten. Every diagnostic for that
  rule MUST carry `skip` equal to the rule's `skip`; `output` is false.
- **Call-site skip** (only on rewriting rules, i.e. rules WITHOUT `skip`): a call
  that the rule would normally rewrite but that is left alone because of how it
  is written or where it sits. Allowed categories: `dynamic-key`,
  `unbound-client`, `context-unproven`, `default-type-unproven` (amendment
  M0.4), `receiver-unproven` (amendment M0.5). A diagnostic on a rewriting rule either
  carries no `skip` (it is rewritten; `output: true` for that language) or
  carries one of those five call-site categories (`output: false` for that
  language).

Consequences: a rewriting rule's skip fixtures (`complex-context`, `dynamic-key`,
`shadowed-client`, `goctx-shadowed-context`) assert their `skip` category again
with `output: false`; any other `skip` value on a rewriting rule is an error;
`ruleId` still equals the case's `rule`. A case may mix languages (e.g. one
rewrites, the other skips) only if each language's `expect` is internally
consistent. Coverage for a rewriting rule counts only cases with `output: true`
and no `skip`.

### M0.2b correction (coverage languages)
In M0 the fixture languages are `js` and `go` only. `java` templates in
rules.yaml are unverified and Java fixtures arrive in M3, so rewrite-coverage
(`spec:tags`, the Go/JS validators and every checker) requires a rewrite case
only for `js` and `go`, never `java`. A `java` entry in `expect` is still
allowed by the schema but is not required or checked beyond file presence.

## M0.3 amendments (idiomatic per ecosystem; override anything above that conflicts)

Decided by the maintainer: each language's tool must be as idiomatic as possible
for its own ecosystem, and nothing should catch a developer off guard. Process
behaviour follows the ecosystem, not a cross-language contract.

### What is shared across languages (the contract)
Rule ids and `rules.yaml`, message text, positions (1-based line, UTF-16 column,
tab = 1), the fingerprint, the baseline and SARIF shapes, and the fixtures.
Anything a CI system aggregates across languages stays shared.

### What is per ecosystem (NOT shared)
Exit codes, flags, severity configuration, config file format, fix workflow and
help text. Do not copy one ecosystem's conventions into another.

- **JS/TS (ESLint plugin and CLI):** follow ESLint exactly.
  - Exit `0` when there are no errors and warnings do not exceed `--max-warnings`;
    `1` when there are errors or warnings exceed `--max-warnings`; `2` for
    config, usage or internal errors. `130` is not specified by ESLint: do not
    document it as part of the contract.
  - `--max-warnings` defaults to `-1` (unlimited), exactly like ESLint. The
    prior default of 0 is withdrawn.
  - Severity is per rule via ESLint configuration (`off`/`warn`/`error`,
    flat config, ESLint 9). The preset `recommended` sets every fldlint rule to
    `warn`; a team gates CI with `error` severity or `--max-warnings 0`.
  - `--quiet` is ESLint's: report errors only; it does not change the exit code.
  - `--fix`, `--fix-dry-run`, `-f/--format` (including json and sarif),
    `--no-inline-config`, and inline `// eslint-disable-next-line fldlint/<rule>`
    suppression work as in ESLint. Prefer ESLint's own suppression mechanisms
    over inventing new ones. fldlint's own baseline file is only for the
    standalone CLI; record in a follow-up issue whether ESLint's bulk
    suppressions should replace it for the plugin.
  - The codemod follows `npx codemod` conventions (verify at M1).
- **Go (go/analysis analyzer, `fldlint`, `fldlint-fix`):** follow the Go
  analyzer ecosystem. Verify the real behaviour of `singlechecker` and
  `unitchecker` binaries empirically (exit code when diagnostics are reported,
  `-fix`, `-json`, `-c`, `-flags`, vet-tool use via `go vet -vettool`) and
  document what you observe. No `--max-warnings` or severity in the Go tools:
  analyzer diagnostics are all equal, and golangci-lint or the user decides
  what fails. SARIF/baseline are optional extras, not default behaviour.
- **Java (OpenRewrite, M3):** follow the Maven/Gradle rewrite plugin
  conventions; unverified, do not specify details now.

### Consequences for existing docs
- ADR 004 ("exit codes follow ESLint") is narrowed to the JS tools; add ADR 008
  "Idiomatic per ecosystem" recording this decision. Pre-release in-place
  amendment is allowed (note "Amended 2026-11").
- exit-codes.md, output-format.md, sarif.md, baseline.md, README.md,
  CONTRIBUTING.md and validation-parity.md must say which parts are shared and
  which are per ecosystem, and must drop any claim that all tools share exit
  codes, flags or `--max-warnings 0`.

## M0.4 amendment: `default-type-unproven` call-site skip

Owner decision. A new call-site skip category, `default-type-unproven`
(severity warning, message "Default value type not proven"), sits alongside
`dynamic-key`, `unbound-client` and `context-unproven`. It means the rule
matched, but the default-value argument is an expression whose type cannot be
proven to match the rule's `valueType` (for example a variable of unknown type),
so no rewrite is made. It is distinct from the rule-level `type-mismatch`, which
is used when the default is provably the wrong type or the rule itself has a
type mismatch between SDKs. Like the other call-site skips it is allowed only
on rewriting rules, requires `output: false`, and is enumerated in
`spec/rules.yaml`, both schemas, and the JS and Go validators. Fixture:
`spec/fixtures/bool-variation/default-type-unproven`.

## M0.5 amendment: `receiver-unproven` call-site skip and planner step order

Owner decision. A new call-site skip category, `receiver-unproven` (severity
warning, message "LaunchDarkly client expression cannot be removed safely"),
becomes the fifth call-site category (M0.2a: `dynamic-key`, `context-unproven`,
`default-type-unproven`, `unbound-client`, `receiver-unproven`). It is allowed
only on rewriting rules, requires `output: false`, and is enumerated in
`spec/rules.yaml`, both schemas, and the JS and Go validators.

The planner step order is shared by every language and first match wins:
rule-level skip, `dynamic-key`, `context-unproven`, `default-type-unproven`,
`unbound-client`, `receiver-unproven`, else rewrite. A call is
`receiver-unproven` when its receiver is not pure, or when rewriting it would
strand a local variable (every rewritable call on that local is then skipped,
never a subset). A default is proven by its static type (Go) or its literal kind (JS/TS);
a default that is not proven, including a literal of the wrong kind, is
`default-type-unproven`; rule-level `type-mismatch` stays only for
rule-level mismatches. Rationale and details: ADR 010. The static-key and step
order definitions are in `spec/contracts/placeholders.md`.

Other points settled here:
- The M0.1 claim that TypeScript tolerates unused locals is true only by
  default. Under `--noUnusedLocals` (TS6133) an unused client local is a compile
  error. JS emits `receiver-unproven` only for impure receivers (it detects only
  identifier receivers today) and keeps the now-unused local.
- The legacy `launchdarkly-node-server-sdk` package is report-only in JS: it is
  detected but never rewritten.

Fixtures: `stranded-local-client`, `stranded-two-calls`, `impure-receiver`,
`field-receiver`, `skip-order-default-before-unbound`, `ldcontext-shared-import`
under `spec/fixtures/bool-variation`.

## M0.6 amendment: toolchain policy (supersedes the M0.1 Toolchain lines for Node, TypeScript and `@types/node`)

Owner decisions, 2026-11-22.

- Node: support the latest Node LTS, always. As of 2026-11-22 Node 26
  "Lithium" is LTS (since 2026-10-28); 24 "Krypton" and 22 "Jod" are in
  maintenance (22 ends 2027-04-30). CI runs the JS test matrix on every
  supported LTS line: `[22, 24, 26]` (`js-test`). `lint` and `spec-check` run
  on a single Node, the latest LTS (26). `engines.node` stays `>=22.12` until
  Node 22 reaches end of life.
- `@types/node` tracks the latest LTS (`^26`). To guard the only real risk
  (code using APIs newer than the floor), CI job `js-types-floor` runs on Node
  22.12.0 itself, installs `@types/node@22.12.0` (the only 22.12-era typings
  release; `^22` would resolve to 22.20.x and accept later APIs such as
  `zstdCompressSync`, @since 22.15) with `npm install --no-save` over the
  locked tree, and runs `npm run typecheck`.
- TypeScript: track the newest TypeScript that typescript-eslint supports.
  `@typescript-eslint/parser@8.73.0` (typescript-eslint latest) peers
  `typescript >=4.8.4 <6.1.0`, so we pin `typescript ~6.0.3` (latest 6.0.x).
  TS 7.x is the native Go port: `require('typescript')` exposes no
  programmatic compiler API, so typescript-eslint cannot support it yet (tsc
  7.0.2 does build and test this repo cleanly). The non-required CI canary
  `typescript-next` installs `typescript@latest` (7.x) and runs typecheck and
  build only. When typescript-eslint supports TS 7, adopt it, drop the
  Dependabot ignore, and retire the canary.
- Dependabot: npm ignores `typescript` major updates (pending the above) and
  `@types/node` non-LTS (odd) majors; dev-dependency minor/patch updates are
  grouped.
- Required checks: `js-test (26)` is to be added to the repo ruleset;
  `js-types-floor` may be added; `typescript-next` must never be required.

## M0.7 amendment: `task` (go-task) replaces `make`

Owner decision, 2026-11-22. The Makefile is deleted; `Taskfile.yml`
(https://taskfile.dev) is the only entry point. Locally `task check` runs all
linting, tests and static analysis, and CI runs the same `task check`, so local
and CI are identical. The "Make targets" section above and ADR text are frozen
history; read every `make X` there as the `task Y` below. go-task is a
developer tool, not a project dependency. CI installs it with
`arduino/setup-task` (go-task pinned to 3.54.0); golangci-lint v2.15.0 runs via `go run` in both places
(the `golangci-lint-action` exception no longer applies).

| Was | Now |
|-----|-----|
| `make help` | `task --list` (also the default task) |
| `make spec-check` | `task spec:check` |
| `make spec-gen` | `task spec:gen` |
| `make fixture-tags` | `task spec:tags` (`ALLOW_EMPTY=1`) |
| `make js-test` | `task js:test` |
| `make go-test` | `task go:test` |
| `make lint` | `task lint` (`js:lint`, `go:lint`) |
| `make lint-js` / `make lint-go` | `task js:lint` / `task go:lint` |
| `make test` | `task test` |
| (new) | `task check` (everything), `task js:types-floor`, `task js:ts-next` |

CI: the path-filter job is removed so required checks always report. One job
`check` runs `task check` on the Node matrix `[22, 24, 26]`; required checks
are `check (22)`, `check (24)`, `check (26)` (and `dco`). `typescript-next`
runs `task js:ts-next` and is never required. This supersedes the job names
(`js-test`, `lint`, `spec-check`, `go-test`, `js-types-floor`) in M0.6.

## M0.8 amendment: approved dependencies list

Owner decisions (dependencies approved during M0 to M3 planning; this keeps the
contract and `CONTRIBUTING.md` consistent, and `CONTRIBUTING.md` carries the
same list). It replaces the "Approved dependencies" section above, which stays
as frozen history. Anything not listed: stop and ask.

- JS (shipped): `eslint` 9.x, `@typescript-eslint/parser`,
  `@typescript-eslint/utils`, `ajv` (+ `ajv-formats`), `yaml`.
- JS (test/build time only): `@types/node` (latest LTS, `^26`; CI also
  typechecks against 22.12.0, M0.6), `typescript` ~6.0, `vitest`,
  `fast-check`, and `esbuild` (only as a build-time devDependency of
  `@fldlint/codemod`).
- Go (shipped): `golang.org/x/tools`, `gopkg.in/yaml.v3`. `testify` stays not
  approved.
- Go (build time only): `github.com/golangci/plugin-module-register`, only in
  the golangci-lint module plugin (issue #18); never imported by the analyzer.
- Java (test/build time only): Maven and the Maven wrapper, JUnit 5.14.4,
  OpenRewrite 8.90.4 (`rewrite-java`, `rewrite-test`, `rewrite-templating`) and
  `rewrite-maven-plugin` 6.46.1 (ADR 009).
- Real-SDK compile-check modules for Go, JS and Java: test time only, kept in
  their own modules, never shipped.
