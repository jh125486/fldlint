# Process behaviour (exit codes, flags, severity)

Status: planned; not yet implemented. Process behaviour is **per ecosystem**
([ADR 008](../../docs/adr/008-idiomatic-per-ecosystem.md)): each tool does what
that language's standard tooling does. There is no cross-language exit-code or
flag contract.

## Shared (what CI aggregators can rely on)

- Rule ids, messages, positions, the fingerprint, the baseline and SARIF
  shapes ([output-format.md](output-format.md), [sarif.md](sarif.md),
  [baseline.md](baseline.md), [fingerprint.md](fingerprint.md)).
- Nothing about exit codes, flag names or how a finding's severity is chosen.
  A pipeline that spans languages should consume SARIF or the JSON output, not
  exit codes.
- `spec/rules.yaml` severities (`skipCategories.*.severity`) are the
  **default** severities of the JS `recommended` preset, not a fixed property
  of a finding. In the JS tools a finding's level is its configured severity.

## JS/TS (ESLint plugin and CLI): follows ESLint exactly

| Code | Meaning |
|------|---------|
| `0`  | No errors, and warnings do not exceed `--max-warnings`. |
| `1`  | At least one error, or warnings exceed `--max-warnings`. |
| `2`  | Configuration, usage or internal error (including a missing explicit `--config`). |

- `--max-warnings` defaults to `-1` (unlimited), exactly like ESLint. With the
  default, warnings never fail a run. (An earlier default of 0 is withdrawn.)
- Severity is per rule through ESLint flat config (ESLint 9 or 10): `off`, `warn` or
  `error`. The `recommended` preset sets every fldlint rule to `warn`.
- To gate CI, a team either sets rules to `error`, or passes `--max-warnings 0`.
- `--quiet` is ESLint's: report errors only. It does not change the exit code.
- `--fix` and `--fix-dry-run` work as in ESLint; the exit code is computed on
  what remains after fixing.
- `-f/--format` includes `json` and `sarif` ([output-format.md](output-format.md),
  [sarif.md](sarif.md)); `--no-inline-config` is supported.
- Inline suppression uses ESLint's own mechanism, e.g.
  `// eslint-disable-next-line fldlint/bool-variation`. Prefer it to inventing
  new suppression syntax.
- fldlint's baseline file ([baseline.md](baseline.md)) applies to the
  standalone CLI only. Whether ESLint's bulk suppressions should replace it for
  the plugin is tracked in a follow-up issue.
- The codemod follows `npx codemod` conventions (to be verified at M1).
- SIGINT: ESLint does not specify an exit code; the conventional 128+2 from the
  OS or shell is not part of this contract.
- Config file format: ESLint flat config (`eslint.config.js`).

## Go (`go/analysis` analyzer; `fldlint`, `fldlint-fix`): follows the Go analysis ecosystem

The tools are `go/analysis` analyzers built with `singlechecker`/`unitchecker`
(or run through `go vet -vettool`, `go fix -fixtool`, golangci-lint, gopls).
**golangci-lint, gopls and `go vet` decide severity and failure policy**, so
the Go tools have no `--max-warnings` and no severity setting: every
diagnostic is equal. SARIF and baseline are optional extras, not default
behaviour.

### Observed behaviour

Observed with Go go1.27.4 darwin/arm64, `golang.org/x/tools v0.53.0` (the
version in `go/go.mod`), using a throwaway analyzer `demo` that reports each
call to `Old()` with one SuggestedFix (rename to `New`). Two binaries were
built from it: `single` (`singlechecker.Main`) and `unit` (`unitchecker.Main`).
Sample packages: `./sample/good` (no `Old()` call) and `./sample/bad` (one).

| Command | Exit | Observed |
|---------|------|----------|
| `./single ./sample/good` | 0 | no output |
| `./single ./sample/bad` | **3** | `.../b.go:7:2: use New` |
| `./single -json ./sample/bad` | 0 | JSON `{pkg: {analyzer: [{posn, end, message, suggested_fixes}]}}` on stdout |
| `./single -json ./sample/good` | 0 | `{}` |
| `./single -c=1 ./sample/bad` | 3 | the finding plus the offending line with 1 line of context |
| `./single -fix ./sample/bad` | 0 | file rewritten (`Old()` became `New()`); no output |
| `./single ./sample/bad` after `-fix` | 0 | clean |
| `./single -fix -diff ./sample/bad` | 0 | unified diff printed, file untouched |
| `./single ./sample/nonexistent` | 1 | load error on stderr (`directory not found`) |
| `./single -bogus ./sample/bad` | 2 | `flag provided but not defined`, then usage |
| `./single -flags` | 0 | JSON list of flags (`V all c diff flags json source tags test v`) |
| `go vet -vettool=$PWD/unit ./sample/good` | 0 | no output |
| `go vet -vettool=$PWD/unit ./sample/bad` | **1** | `sample/bad/b.go:7:2: use New` (stderr) |
| `go vet -vettool=$PWD/unit -json ./sample/bad` | 0 | same JSON shape as above |
| `go vet -vettool=$PWD/unit -c=1 ./sample/bad` | 1 | finding plus context line |
| `go vet -vettool=$PWD/unit -fix ./sample/bad` | 0 | file rewritten |
| `go vet -vettool=$PWD/unit -demo=false ./sample/bad` | 0 | analyzer disabled by its name flag |
| `$PWD/unit -flags` | 0 | JSON flag list; includes a per-analyzer `demo` flag |
| `go fix -fixtool=$PWD/unit ./sample/bad` | 0 | file rewritten |
| `go fix -diff -fixtool=$PWD/unit ./sample/bad` | **1** | unified diff printed, file untouched; go1.27.4 only: `go help fix` documents non-zero on a non-empty diff (go1.26.0 exits 0, see below) |
| `$PWD/unit ./sample/bad` | 1 | refuses direct use: `invoking "go tool unit" directly is unsupported; use "go unit"` |

Consequences for the Go tools:

- Exit codes are whatever the checker framework gives: `singlechecker` exits
  `3` when diagnostics are reported, `go vet -vettool` exits `1`; `-json` exits
  `0` even with findings; a package that fails to load exits `1`; a bad flag
  exits `2`. Do not map these onto ESLint's codes, and do not override them.
- `-fix` rewrites in place and exits `0`; `-fix -diff` previews. There is no
  `--fix-dry-run`.
- `-json` is the framework's format (positions as `file:line:col` strings,
  byte offsets for edits); it is not fldlint's JSON format. A `-format`
  sarif option, if offered, is an extra.
- The unitchecker binary only runs under `go vet -vettool` / `go fix -fixtool`.
- The `go fix -diff` exit code is toolchain-dependent (1 on go1.27.4, 0 on
  go1.26.0): check for a non-empty diff instead of relying on the exit code.

Commands were run from a scratch module outside this repository; the
throwaway code is not part of the repo.

### Observed with the real analyzer

The same behaviour, re-observed with the real `fldlint` (`singlechecker`) and
`fldlint-fix` (`unitchecker`) binaries built from `go/cmd/*` (go1.27.4
darwin/arm64, `golang.org/x/tools v0.53.0`). Target: a throwaway module with
the conformance fixture `bool-variation/aliased-import` against the SDK stubs
(one rewritable `BoolVariation` call). The fixed output equals the fixture's
`output.go.golden`. The "go1.26.0" column reruns the vet/fix rows with
`GOTOOLCHAIN=go1.26.0` (the `go.mod` floor) driving the same binaries, which
were built with go1.27.4 (all rows re-run; only `go fix -diff` differs).

| Command | Exit (go1.27.4) | Exit (go1.26.0) | Observed |
|---------|-----------------|-----------------|----------|
| `fldlint ./...` | **3** | 3 | `file:21:11: Migrate boolean flag "dark-mode" to OpenFeature` |
| `fldlint -json ./...` | 0 | 0 | framework JSON; `category` is the rule id, with `suggested_fixes` |
| `fldlint -fix ./...` | 0 | 0 | file rewritten to the golden; a rerun is clean (exit 0) |
| `fldlint -fix -diff ./...` | 0 | 0 | unified diff, file untouched |
| `fldlint ./nonexistent` | 1 | 1 | `directory not found`, `analysis skipped due to errors in package` |
| `fldlint -bogus ./...` | 2 | 2 | `flag provided but not defined`, then usage |
| `go vet -vettool=fldlint-fix ./...` | **1** | 1 | `flags/input.go:21:11: Migrate boolean ...` |
| `go vet -vettool=fldlint-fix -json ./...` | 0 | 0 | same JSON shape |
| `go vet -vettool=fldlint-fix -fix ./...` | 0 | 0 | file rewritten to the golden |
| `go vet -vettool=fldlint-fix -fldlint=false ./...` | 0 | 0 | analyzer disabled by its name flag |
| `go fix -fixtool=fldlint-fix ./...` | 0 | 0 | file rewritten to the golden; `go vet` is clean afterwards |
| `go fix -diff -fixtool=fldlint-fix ./...` | **1** | **0** | diff printed, file untouched; see below |
| `fldlint-fix ./...` (direct) | 1 | 1 | `invoking "go tool fldlint-fix" directly is unsupported; use "go fldlint-fix"` |

`go fix -diff` exits non-zero on a non-empty diff on go1.27.4 only: on go1.26.0
it printed the same diff and exited `0`, and `go help fix` there has no
"non-zero" sentence. Do not rely on the exit code of `go fix -diff` on the
`go.mod` floor (1.26.0); check for a non-empty diff instead.

Still unverified: gopls, and any Go version other than go1.27.4 and go1.26.0.
The golangci-lint plugin is covered in the next section.

### golangci-lint module plugin

Observed (not constraints: the toolchain versions are just what was used) with
golangci-lint v2.15.0 (built with go1.27.0, run on go1.27.4) and
`github.com/golangci/plugin-module-register` v0.1.2, running go1.27.4
darwin/arm64. `golangci-lint custom` built `custom-gcl` from
`go/golangci/testdata/.custom-gcl.yml`; the sample is
`go/golangci/testdata/smoke` (one rewritable `BoolVariation` call).

| Command | Exit | Observed |
|---------|------|----------|
| `golangci-lint custom -v` | 0 | custom binary built (clones golangci-lint, needs network) |
| `./custom-gcl run ./...` | **1** | `smoke/flags/flags.go:21:11: Migrate boolean flag "dark-mode" to OpenFeature (fldlint)`, source line, `1 issues: * fldlint: 1` |
| `./custom-gcl run --fix ./...` | 0 | `0 issues.`; file rewritten (adds `"context"`, replaces the call with `client.BooleanValue(context.TODO(), ...)`, drops the unused `ldcontext` import) |
| `./custom-gcl run ./...` after `--fix` | 0 | `0 issues.` |
| `./custom-gcl run --output.json.path=stdout ./...` | 1 | JSON `Issues[]` with `SuggestedFixes` (byte-offset `TextEdits`) |

So suggested fixes do apply under `golangci-lint run --fix`. golangci-lint
reports every diagnostic as one severity and exits `1` on any finding (not
`3`).

Unverified: gopls, and anything on Go versions other than go1.27.4.

## Java (OpenRewrite, M3): placeholder

Will follow Maven/Gradle rewrite plugin conventions (e.g. `rewrite:run`,
`rewrite:dryRun`). **Unverified; not specified until M3.**

## Deliberate difference from flaglint

flaglint (ADR 010 in flaglint-js, ADR 003 in flaglint-go) defines `scan` and
`audit` as inventory commands that never exit `1` for findings, reserves `1`
for `validate`, and uses `3` for internal errors. fldlint is a linter and each
tool follows its own ecosystem instead. See
[ADR 004](../../docs/adr/004-exit-codes-follow-eslint.md) and
[ADR 008](../../docs/adr/008-idiomatic-per-ecosystem.md). Baseline files and
fingerprints are not interchangeable with flaglint's.
