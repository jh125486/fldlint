# AGENTS.md

Instructions for coding agents (Codex, Claude Code, others) working in
`jh125486/fldlint`. Read [CONTRIBUTING.md](CONTRIBUTING.md) first: it holds the
task table, approved dependencies, Go test conventions and spec-change rules.
This file adds agent-specific rules and does not repeat those.

## Who merges

The owner (`jh125486`) merges. An agent merges only when the owner explicitly
authorizes it. Squash merge is the only allowed method. Never enable
auto-merge or squash auto-merge.

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

Run from the repo root; `task --list` shows all tasks (table in CONTRIBUTING.md).

- `task check`: the full gate (see the `check` task in `Taskfile.yml`).
- `task check:node`: Node-dependent subset.
- `task parity`: JS and Go fingerprint/baseline output must be byte-identical.
- `task spec:gen`, `task spec:check`, `task spec:tags`.
- `task java:lint`, `task java:test` (JDK 21 and `unzip` required).
- `task workflows:lint`: only when you change `.github/workflows`.
- No JDK: `FLDLINT_SKIP_JAVA=1 task check` skips the Java tasks. The CI
  `check (N)` matrix legs run with `FLDLINT_SKIP_JAVA=1`; the Node 22 leg runs
  `task check`, the others `task check:node`. Java runs in a separate
  path-filtered job.
- Optional, if your `task` shim lacks the needed version:
  `mise x task@<ver> -- task <name>`.

Docs-only changes need no task run.

## Workflow rules

- Always work in a git worktree on a branch. Never commit to `main`.
- Open a PR. Merge `origin/main` into your branch before pushing (the ruleset
  requires the branch to be up to date).
- All checks required by the repo ruleset must pass (read the ruleset, do not
  assume a list), and the PR title check should pass too. Review threads must
  be resolved before merge.
- Owner policy: sign commits (`-S`, gpg or ssh) in addition to the DCO
  sign-off (`-s`). If signing or ssh fails, retry a few times. Never disable
  signing, switch remotes or otherwise work around a signing failure; report
  it to the owner.
- Actions minutes are scarce: do not re-trigger reviews or update the branch
  without need, and avoid extra pushes.

## Review loop

1. After opening the PR, wait for every configured review bot to review the
   current head commit, not an earlier one.
2. Address every comment. Fix with a code commit, then reply to AND resolve
   each thread (a reply alone leaves it unresolved).
3. Batch all fixes of a round into one push; each push re-runs CI and
   re-triggers the reviews.
4. Repeat until a round comes back clean.
5. Merge (when authorized, see above) only when every configured review bot
   has reviewed the current head, required checks pass, merge state is CLEAN
   and no threads are unresolved.

## Issues

Assign the issue to the owner (`jh125486`) when you start work.

## Orchestration

- Plan before coding. Implement in a worktree.
- If your harness supports subagents or model tiers: use the cheapest capable
  model for mechanical, read-only polling and status checks; get a
  fresh-context review of the diff before opening the PR (strongest model for
  parsers, rewrite engines and security-sensitive code; a mid-tier model for
  CI config, docs and dependency changes).
- The reviewer and the implementer must be different sessions.

## Dependencies

Adding any dependency, GitHub Action or tool needs owner approval first, and
it must be the latest stable version. Pin Actions as the repo already does
(see `.github/workflows/ci.yml`).

## Decisions

When a decision belongs to the owner (naming, versioning, publishing, repo
settings, new dependencies), stop and ask one explicit question with options
and a recommendation. Do not bury it in status text. Do not re-ask resolved
decisions: check `docs/adr/` first.

## Go tests

Follow "Go test conventions" in CONTRIBUTING.md (and the `go-test-conventions`
skill if your harness has it).

## Correctness invariants

- Rewrite a call only when the flag key is static, the value type is known,
  any context conversion is provable, and the OpenFeature client binding is
  proven in scope. Otherwise report it with a skip category and do not edit.
  Skip order and receiver rules: ADR 006 and ADR 010.
- Positions: 1-based line, `col` in UTF-16 code units, tab counts as 1
  (`spec/contracts/output-format.md`).
- Fingerprint and baseline contract: `spec/contracts/` (`fingerprint.md`,
  `baseline.md`); JS and Go must stay byte-identical (`task parity`).

## Never, without owner approval

- Touch repo settings, branch protection or secrets.
- Publish packages or push tags.
- Work around a permission denial; report it instead.
