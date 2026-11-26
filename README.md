# fldlint

fldlint finds LaunchDarkly server-SDK flag evaluations in JavaScript/TypeScript,
Go and Java, and migrates the ones it can prove safe to OpenFeature. Calls it
cannot prove safe are reported with a skip category and left alone.

> Status: **not released.** There are no releases and no tags. Nothing is
> published to npm, Maven Central or the Go module proxy, and the repository is
> private until the first release. Use it from a checkout of this repository.

**This is a separate project from [flaglint](https://github.com/flaglint)
(similar name).** It is derived in part from flaglint-js and flaglint-go under
the MIT license; see [NOTICE](NOTICE) and [ADR 005](docs/adr/005-attribution-and-porting-policy.md).
It is not affiliated with or endorsed by the flaglint project.

LaunchDarkly is a trademark of its owner. fldlint is an independent project
and is not affiliated with, sponsored by or endorsed by LaunchDarkly or the
OpenFeature project.

## Status by ecosystem

| Ecosystem | What exists on `main` | Published |
|-----------|-----------------------|-----------|
| JS/TS | ESLint plugin ([`eslint-plugin-fldlint`](js/packages/eslint-plugin/README.md)), codemod package ([`@fldlint/codemod`](js/packages/codemod/README.md)) over a shared planner (`@fldlint/core`). The ESLint plugin runs through `eslint` itself. There is no separate fldlint CLI planned for the first release; `@fldlint/cli` is an unimplemented placeholder. | No: every package is `private`; npm names are planned |
| Go | `go/analysis` analyzer (`go/analyzer`) with suggested fixes, and two binaries: `go/cmd/fldlint` (standalone) and `go/cmd/fldlint-fix` (`go vet -vettool`). Handles go-server-sdk v7 only. | No: not tagged, not on the Go proxy |
| Java | OpenRewrite recipes (`io.github.jh125486.fldlint`): four rewrite recipes (`BoolVariationToOpenFeature`, `StringVariationToOpenFeature`, `NumberVariationToOpenFeature`, `IntVariationToOpenFeature`, grouped as `MigrateLaunchDarklyToOpenFeature`) and a search recipe, `FindLaunchDarklyUsage`, which is `dryRun`-only. Details in [java/README.md](java/README.md). | No: the artifact `io.github.jh125486:fldlint-rewrite` is planned, not on Maven Central |

Each ecosystem has its own detection engine; they share data, not code
([ADR 002](docs/adr/002-spec-as-data-not-shared-engine.md)), and each tool
follows its ecosystem's conventions ([ADR 008](docs/adr/008-idiomatic-per-ecosystem.md)):
ESLint for JS/TS (rules default to `warn`; gate CI with `error` severity or
`--max-warnings 0`), `go/analysis` for Go, the OpenRewrite Maven/Gradle plugins
for Java. See [exit-codes.md](spec/contracts/exit-codes.md).

## How rewrite safety works

A call is rewritten only when the engine can prove all of these:

1. the flag key is a static string;
2. the default value's type is known and matches the target method;
3. the LaunchDarkly context is a simple single-kind literal
   ([ADR 006](docs/adr/006-context-conversion-skipped-unless-provable.md));
4. an OpenFeature client binding is in scope, and removing the LaunchDarkly
   receiver is safe ([ADR 010](docs/adr/010-planner-step-order-and-receiver-removal.md)).

Every engine applies one fixed step order, first match wins, so the same call
gets the same category everywhere. Anything unproven is reported, not rewritten,
with one of the skip categories in [`spec/rules.yaml`](spec/rules.yaml):
`detail`, `bulk`, `dynamic-key`, `unbound-client`, `type-mismatch`,
`context-unproven`, `default-type-unproven`, `receiver-unproven`. Some rules
(detail calls, bulk evaluation, migration flags, and JSON values in Go and
Java) are report-only; JS `json-variation` is rewritable.

## Try it from source

Requirements: Node `>=22.13`, the Go version in `go/go.mod`, and JDK 21 for
Java. Dev tasks run through [go-task](https://taskfile.dev); `task --list` shows
them all (table in [CONTRIBUTING.md](CONTRIBUTING.md)).

- ESLint plugin: build it (`npm ci && npm run build` in `js/`) and install it
  from the path; see [its README](js/packages/eslint-plugin/README.md).
- Codemod: see [its README](js/packages/codemod/README.md) for the
  `npx codemod workflow run` invocation against a checkout.
- Go: `cd go && go run ./cmd/fldlint ./...` runs the analyzer standalone
  (`singlechecker`); `go build -o /tmp/fldlint-fix ./cmd/fldlint-fix` then
  `go vet -vettool=/tmp/fldlint-fix ./...` runs it through `go vet`.
- Java: `./mvnw -B verify` in `java/` (or `task java:test`) builds and runs the
  tests; `task java:smoke` runs the recipes through `rewrite-maven-plugin`
  `dryRun` against `java/smoke`.
- Everything: `task test` runs the JS, Go and Java tests; `task check` runs what
  the required CI `check` job runs; `task spec:check` validates the spec and
  checks generated code for drift.

## Shared spec

[`spec/`](spec/) is the source of truth: `rules.yaml` (rule ids, messages,
per-language methods and templates, skip categories), JSON Schemas, fixtures
(`case.yaml` per case), and [contracts](spec/contracts/) for output format,
fingerprint, baseline, SARIF, placeholders and exit codes. Only the JS loader
reads the YAML; it emits `spec/generated/spec.json`, which Go and Java read
([ADR 011](docs/adr/011-parse-yaml-once-emit-canonical-json.md)). Decisions are
recorded in [docs/adr/](docs/adr/).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (tasks, conventions, sign-off) and
[AGENTS.md](AGENTS.md) (rules for coding agents). Licensed under the
[MIT License](LICENSE); attribution is in [NOTICE](NOTICE).
