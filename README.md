# fldlint

> Status: **pre-alpha.** The repository is private until the JS milestone (M1)
> ships. Everything below describes the plan; nothing here is implemented yet
> unless it says so.

fldlint (planned) finds LaunchDarkly server-SDK flag evaluations in JS/TS, Go
and later Java, and migrates the safe ones to OpenFeature.

**This is a separate project from [flaglint](https://github.com/flaglint)
(similar name).** It is derived in part from flaglint-js and flaglint-go under
the MIT license; see [NOTICE](NOTICE) and [ADR 005](docs/adr/005-attribution-and-porting-policy.md).
It is not affiliated with or endorsed by the flaglint project.

LaunchDarkly is a trademark of its owner. fldlint is an independent project
and is not affiliated with, sponsored by or endorsed by LaunchDarkly or the
OpenFeature project.

## Safety model

A call is rewritten only when all of these hold:

1. the flag key is a static string;
2. the types involved are known (value type and default type agree);
3. the OpenFeature client binding is proven in scope.

Everything else is reported, not rewritten: dynamic keys, detail calls, bulk
evaluation, unbound clients, type mismatches and unproven context conversion.
These are the skip categories in `spec/rules.yaml`.

Context conversion is the strictest part: the LaunchDarkly context is rewritten
only when it is a simple single-kind literal (JS `{ key, kind?: "user" }`, Go
`ldcontext.New(...)`), in both languages ([ADR 006](docs/adr/006-context-conversion-skipped-unless-provable.md),
[placeholders](spec/contracts/placeholders.md)). Go rewrites must compile; TS
output may keep an unused client.

## Layout

```
spec/       rules.yaml, JSON Schemas, fixtures, contracts,
            generated/spec.json (the one JSON form Go and Java read)
js/         npm workspaces: spec, eslint-plugin, codemod, cli
go/         spec bindings, conformance harness, analyzer
java/       placeholder until M3
docs/adr/   architecture decision records
```

Each language has its own detection engine; they share data, not code
([ADR 002](docs/adr/002-spec-as-data-not-shared-engine.md)). Output shapes
and data contracts live in [spec/contracts](spec/contracts/).

Each tool is idiomatic for its ecosystem ([ADR 008](docs/adr/008-idiomatic-per-ecosystem.md)).
Shared: rule ids, messages, positions, fingerprint, baseline and SARIF shapes,
fixtures. Per ecosystem: exit codes, flags, severity configuration and fix
workflow. JS/TS behaves like ESLint (rules default to `warn`; gate CI with
`error` severity or `--max-warnings 0`); Go behaves like a `go/analysis` tool
(`go vet -vettool`, golangci-lint); Java will follow the OpenRewrite
Maven/Gradle plugins. See [exit-codes.md](spec/contracts/exit-codes.md).

## Status and requirements

Only the M0 scaffold exists: the spec (rules, schemas, fixtures, contracts),
generated rule tables, a conformance harness that loads and validates every
case and reports it as pending, and CI. No detection engine, rewrite, ESLint
rule, codemod or CLI is implemented; the contracts under `spec/contracts/` are
plans.

- Node `>=22.12` (CI tests every supported LTS line: 22, 24 and 26; Node 20 is not supported).
- Go: the version in `go/go.mod`.
- CodeQL and dependency review run in CI only when the repository variable
  `GHAS_ENABLED` is `'true'`; it is unset while the repo is private and is to
  be set when the repo goes public.

Run `task --list` for tasks; `task test` runs the JS and Go tests,
`task check` runs everything the required CI `check` job runs (the non-required `typescript-next` canary is `task js:ts-next`), and `task spec:check` validates the spec and checks generated code for drift.

## Milestones

| Milestone | Scope | State |
|-----------|-------|-------|
| M0 | Scaffold: spec, schemas, codegen, conformance harness (cases pending), CI, contracts | in progress |
| M1 | JS/TS engine: ESLint plugin, codemod, CLI | planned |
| M2 | Go engine: `go/analysis` analyzer | planned |
| M3 | Java: OpenRewrite recipe | planned |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Licensed under the [MIT License](LICENSE).
