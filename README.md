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

## Layout

```
spec/       rules.yaml, JSON Schemas, fixtures, contracts   (shared data)
js/         npm workspaces: spec, eslint-plugin, codemod, cli
go/         spec bindings, conformance harness, analyzer
java/       placeholder until M3
docs/adr/   architecture decision records
```

Each language has its own detection engine; they share data, not code
([ADR 002](docs/adr/002-spec-as-data-not-shared-engine.md)). Output and
behavior contracts live in [spec/contracts](spec/contracts/).

## Milestones

| Milestone | Scope | State |
|-----------|-------|-------|
| M0 | Scaffold: spec, schemas, codegen, conformance harness (cases pending), CI, contracts | in progress |
| M1 | JS/TS engine: ESLint plugin, codemod, CLI | planned |
| M2 | Go engine: `go/analysis` analyzer | planned |
| M3 | Java: OpenRewrite recipe | planned |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Licensed under the [MIT License](LICENSE).
