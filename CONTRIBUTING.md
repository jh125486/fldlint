# Contributing

## Commits and PRs

- Sign off every commit (Developer Certificate of Origin): `git commit -s`.
- PR titles and commit messages use conventional commits (`feat:`, `fix:`,
  `docs:`, `chore:`, `test:`, ...).
- Do your work on a branch (ideally a git worktree); open a PR rather than
  pushing to `main`.

## Make targets

| Target | Does |
|--------|------|
| `make help` | Lists targets |
| `make spec-check` | Validates `rules.yaml` and every `case.yaml`, runs both codegens and fails on drift, then runs `fixture-tags` |
| `make spec-gen` | Regenerates the committed JS and Go rule tables |
| `make fixture-tags` | Checks mandatory shape-tag coverage (`bool-variation` only) and a rewrite case per rule for each language with a template |
| `make js-test` | Installs and runs JS tests (`npm test` in `js/`) |
| `make go-test` | Go tests with the race detector (`go test ./...` in `go/`) |
| `make lint` | `lint-go` and `lint-js` |
| `make test` | `js-test` and `go-test` |

CI calls only these targets. Requires Node `>=22.12` (CI runs 22 and 24) and
the Go version in `go/go.mod`. CodeQL and dependency review run only when the
repository variable `GHAS_ENABLED` is `'true'` (`vars.GHAS_ENABLED == 'true'`);
a maintainer sets it (Settings > Secrets and variables > Actions > Variables)
when the repo goes public. If you change `spec/rules.yaml`, run
`make spec-gen` and commit the generated files.

## Dependencies

Only approved dependencies may be used.

- JS: `eslint` 9.x, `@types/node` ^22, `@typescript-eslint/parser`, `@typescript-eslint/utils`,
  `ajv` (+ `ajv-formats`), `yaml`, `vitest` ^5.1.1, `typescript`.
- Go: `golang.org/x/tools`, `gopkg.in/yaml.v3`. `testify` is not approved.

Anything else: ask first, in an issue or on the PR, before adding it.

## Go test conventions

- Table-driven tests, with `t.Parallel()` in tests and subtests.
- Blackbox tests in `package foo_test`; use `package foo` in a file named
  `*_internal_test.go` only when internals are needed.
- One test function per exported function.
- Use `t.Context()`, not `context.Background()`.
- Use `testing/synctest` for anything timing-related.
- Stdlib `testing` only.

## Porting from flaglint

Port targeted algorithms only. Keep an SPDX header and a
`Derived from flaglint-<x> <path> @<sha>` comment, and update `NOTICE`
([ADR 005](docs/adr/005-attribution-and-porting-policy.md)).

## Spec changes

Rules, schemas, fixtures and contracts are the source of truth. Change the
spec first, then the engines. ADRs are immutable once accepted; supersede them
with a new one (the pre-release amendments noted in the ADR index are the one
exception).

- The JS and Go validators must accept and reject the same inputs; add a
  negative test to each suite with any new check (the suites keep their own;
  they do not share probes)
  ([validation-parity.md](spec/contracts/validation-parity.md)).
- Shape tags are mandatory for `bool-variation` only; every rewriting rule
  needs at least one rewrite case for each language that has a template
  ([ADR 007](docs/adr/007-shape-tag-coverage-reference-rule-only.md)).
- Positions are 1-based, with `col` in UTF-16 code units
  ([output-format.md](spec/contracts/output-format.md#positions)).
- Rewrites that change the context argument must follow
  [placeholders.md](spec/contracts/placeholders.md).
- JS fixtures import `@launchdarkly/node-server-sdk`, not the legacy
  `launchdarkly-node-server-sdk`.
