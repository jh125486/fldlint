# AGENTS.md

Instructions for coding agents (Codex, Claude Code, others) working in
`jh125486/fldlint`. Humans: see [CONTRIBUTING.md](CONTRIBUTING.md), which this
file does not duplicate (task table, approved dependencies, spec-change rules).

## Repo map

- `js/`: JS/TS packages (npm workspace). Owns the only YAML loader/validator.
- `go/`: Go analyzer and tools (`go/internal/spec`, `go/internal/fingerprint`).
- `java/`: OpenRewrite recipes, Maven wrapper `java/mvnw` (JDK 21).
- `spec/`: shared source of truth.
  - `rules.yaml`, `schema/`, `fixtures/` (`case.yaml` per case), `probes/yaml/`
  - `contracts/`: output format, fingerprint, baseline, SARIF, placeholders,
    exit codes, validation parity
  - `generated/spec.json`: generated, committed
- `docs/adr/`: accepted decisions (immutable; supersede, do not edit).

Change the spec first, then the engines. Each language follows its own
ecosystem's idioms (ADR 008); only the shared contract is identical.

## Compile-once spec (ADR 011)

The JS loader is the only code that reads YAML. Go and Java read
`spec/generated/spec.json`. Never hand-edit `spec/generated/spec.json`,
`js/packages/spec/src/rules.gen.ts`, `go/internal/spec/rules_gen.go`,
`go/internal/spec/testdata/spec.json` or
`java/src/main/java/io/github/jh125486/fldlint/spec/RulesGen.java`. Edit the
YAML, run `task spec:gen`, commit the output. `task spec:check` fails on drift.

## Commands

Run from the repo root. `task --list` shows all tasks.

- `task check`: full gate (spec:check, lint, test, parity, js:types-floor).
- `task check:node`: Node-dependent subset (js:lint, js:test, parity).
- `task parity`: JS and Go fingerprint/baseline output must be byte-identical.
- `task spec:gen`, `task spec:check`, `task spec:tags`.
- `task java:lint`, `task java:test` (JDK 21 and `unzip` required).
  There is no `java:smoke` task.
- `task go:test`, `task go:lint`, `task js:test`, `task js:lint`.
- `task workflows:lint`: only when you change `.github/workflows` (needs `uv`).
- No JDK: `FLDLINT_SKIP_JAVA=1 task check` skips the Java tasks. CI's
  required `check` job does this; Java runs in a separate path-filtered job.
- If the `task` shim lacks a version: `mise x task@<ver> -- task <name>`
  (CI pins go-task 3.54.0).

Docs-only changes need no task run.

## Workflow rules

- Always work in a git worktree on a branch. Never commit to `main`.
- Open a PR. Never enable auto-merge or squash auto-merge. Merge is squash
  only.
- Merge `origin/main` into your branch before pushing; the branch must be up
  to date with `main`.
- Required checks: `check (22)`, `check (24)`, `check (26)`, `validate`,
  `dco`.
- PR titles and commit messages use conventional commits (`feat:`, `fix:`,
  `docs:`, `chore:`, `test:`).
- Sign off every commit (DCO): `git commit -s`. Commits are also
  cryptographically signed. If signing or ssh fails, retry a few times. Never
  disable signing, switch remotes, or otherwise work around it; report it to
  the owner.
- Actions minutes are scarce: batch changes, avoid extra pushes.

## Review loop

1. After opening the PR, wait for Copilot, CodeRabbit and Greptile to review
   the current head commit (not an earlier one).
2. Address every comment. Fix with a code commit, then reply to AND resolve
   each thread (a reply alone leaves it unresolved).
3. Batch all fixes of a round into one push. Each push re-runs CI and
   re-triggers the reviews.
4. Do not post `@greptileai review` or run `gh pr update-branch` unless
   needed.
5. Repeat until a round comes back clean.
6. Merge only when Greptile approved the current head, required checks pass,
   merge state is CLEAN and no threads are unresolved. The owner merges or
   explicitly authorizes the merge.

## Issues

Assign the issue to the owner (`jh125486`) when you start work. Issues track
the roadmap in #6.

## Orchestration

- Plan before coding. Implement in a worktree.
- Use the cheapest capable model for mechanical, read-only polling and status
  checks.
- Get a fresh-context review of the diff before opening the PR. Use the
  strongest model for parsers, rewrite engines and security-sensitive code; a
  mid-tier model for CI config, docs and dependency changes.
- The reviewer and the implementer must be different sessions.

## Dependencies

Adding any dependency, GitHub Action or tool needs owner approval first, and
it must be the latest stable version. Pin Actions as the repo does: full
commit SHA with a version comment (see `.github/workflows/ci.yml`). The
approved list is in CONTRIBUTING.md.

## Decisions

When a decision belongs to the owner (naming, versioning, publishing, repo
settings, new dependencies), stop and ask one explicit question with options
and a recommendation. Do not bury it in status text. Do not re-ask resolved
decisions: check `docs/adr/` first (for example ADR 010 fixes the planner
step order).

## Go tests

Follow CONTRIBUTING.md "Go test conventions":

- Table-driven; `t.Parallel()` in tests and subtests.
- Blackbox in `package foo_test`; whitebox only in `*_internal_test.go` with
  `package foo`.
- One test function per exported function.
- `t.Context()`, not `context.Background()`.
- `testing/synctest` for anything timing-related.
- Stdlib `testing` only (no testify).

## Correctness invariants

- Rewrite a call only when the flag key is static, the value type is known,
  any context conversion is provable, and the OpenFeature client binding is
  proven in scope. Otherwise report it with a skip category and do not edit
  (ADR 006, ADR 010). Step order, first match wins: rule-level skip,
  `dynamic-key`, `context-unproven`, `default-type-unproven`,
  `unbound-client`, `receiver-unproven`, rewrite.
- Never delete declarations, insert `_ = x`, or add `Close()` calls (ADR 010).
- Positions: 1-based line, `col` in UTF-16 code units, tab counts as 1
  (`spec/contracts/output-format.md`).
- Fingerprint and baseline formats are defined in `spec/contracts/`
  (`fingerprint.md`, `baseline.md`, `fingerprint-vectors.json`); JS and Go
  must stay byte-identical (`task parity`).

## Never, without owner approval

- Touch repo settings, branch protection or secrets.
- Publish packages or push tags.
- Work around a permission denial; report it instead.
