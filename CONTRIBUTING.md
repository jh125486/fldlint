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
| `make spec-check` | Validates `rules.yaml` and every `case.yaml`, runs both codegens and fails on drift, checks fixture tag coverage |
| `make spec-gen` | Regenerates the committed JS and Go rule tables |
| `make js-test` | JS tests (`npm test` in `js/`) |
| `make go-test` | Go tests (`go test ./...` in `go/`) |
| `make lint` | Linters |
| `make test` | Everything |

CI calls only these targets. If you change `spec/rules.yaml`, run
`make spec-gen` and commit the generated files.

## Dependencies

Only approved dependencies may be used.

- JS: `eslint` 9.x, `@typescript-eslint/parser`, `@typescript-eslint/utils`,
  `ajv` (+ `ajv-formats`), `yaml`, `vitest`, `typescript`.
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
spec first, then the engines. ADRs are immutable; supersede them with a new one.
