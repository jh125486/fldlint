# Real-repo corpus

Issue #21, Part 2. Runs the fldlint tools over pinned public repos that use the
LaunchDarkly server SDK, in report mode first, then applies the fixes in a temp
copy and builds it. Read-only with respect to the world: it never pushes, opens
PRs or comments upstream, copies no code into this repository (only
`url` + `sha` live in [`spec/corpus.json`](../../spec/corpus.json)) and deletes
its temp dirs.

```sh
task corpus            # all repos, compare with spec/corpus-snapshot.json
task corpus:js         # or corpus:go, corpus:java
task corpus:selftest   # apply-fix-then-build on local fixtures (see below)
task corpus:update     # re-run everything and rewrite spec/corpus-snapshot.json
node scripts/corpus/run.mjs run --only hello-go,ld-sample-app-go   # a subset
```

Needs network, `git`, Node (built-ins only; the corpus is JSON, and `npm ci` in
`js/` is needed only to build the ESLint plugin for the JS repos), Go (the repo's toolchain), and JDK 21 for Java. Results
go to `tmp/corpus/result.json`; a post-fix build failure also leaves
`tmp/corpus/<repo>.fix.patch`. Set `RUNNER_TEMP` to choose the temp root and
`CORPUS_KEEP_PATCHES=1` to keep every fix patch.

## What runs per repo

Each repo gets its own deadline (`timeout` in `spec/corpus.json`, seconds, for
clone, dependencies, tools and both builds; the process group is killed at the
deadline and the repo is reported as `timeout`). The clone is
`git fetch --depth <depth> origin <sha>` of exactly the pinned commit.

| Lang | Report | Fix (temp copy) | Build before and after the fix |
|------|--------|-----------------|--------------------------------|
| js | ESLint flat config with the plugin built from this checkout and `@typescript-eslint/parser`, `--format json` ([walkthroughs](../../docs/walkthroughs.md)) | `eslint --fix` | `tsc --noEmit` when a `tsconfig.json` exists (after `npm install --ignore-scripts`) |
| go | `fldlint -json ./...` built from `go/`, run in the repo's own module (`go mod download`, or `go mod tidy` when no `go.sum` is committed) | `fldlint -fix ./...` | `go build ./...` |
| java | `rewrite-maven-plugin:dryRun` with the recipes of [`java/smoke`](../../java/smoke/README.md) and `-Drewrite.exportDatatables=true` (the `LaunchDarklyUsage` table) | `rewrite:run` with `MigrateLaunchDarklyToOpenFeature` only | `mvn -q compile` |

Dependency prep (npm install, go mod tidy) is committed in the temp repo before
the fix, so the fix diff contains only fldlint's edits.

## Output

Per repo: `status` (`ok`, `error`, `timeout`), `counts` (`total`; `rewrites`
by rule; `skips` by skip category; `noFix` by `rule:category`), `fixedFiles`,
`build.pre` and `build.post` (`ok`, `exit`, `log` tail, or `skipped` with a
reason) and `durationMs`. The committed `spec/corpus-snapshot.json` is the same
minus the volatile fields (times, logs).

A finding is a **rewrite** when it carries neither `[skip: ...]` nor
`[no-fix: ...]`. Rewrites only happen where the OpenFeature client binding is
provable in scope, so a repo that has never adopted OpenFeature reports skips
only; zero rewrites is normal for most of the corpus.

## Pass / fail

`node scripts/corpus/run.mjs compare` (run by `task corpus` and the weekly
workflow) fails only when:

1. **A rewrite count rises.** For any repo and rule, `rewrites[rule]` is
   greater than in the committed snapshot, or a repo missing from the snapshot
   has any rewrite. A rise means the tool now edits calls it used to leave
   alone; that needs a fixture under `spec/fixtures/` that explains it (the
   new behaviour is intended and covered) before a maintainer runs
   `task corpus:update` and commits the new snapshot. The snapshot update is
   the explicit acknowledgement: CI never updates it.
2. **A post-fix build breaks** (`build.post.ok === false` while `build.pre.ok`
   was true). This is a **safety bug**: fldlint produced code that does not
   build. Do not update the snapshot to hide it. Add a minimal reproduction
   under `spec/fixtures/`, fix the engine, then re-run. If the build was
   already broken before the fix, there is no verdict and only a warning.

Everything else is informational and printed as `note:` or `warning:`: a
rewrite count that falls, skip or no-fix counts that change, a changed pinned
SHA, a repo that errored or timed out (network flakes must not fail the job).

## Self-test

No corpus repo produces a rewrite (none has an OpenFeature client in scope), so
the apply-fix-then-build path is exercised separately by
`task corpus:selftest` (`run.mjs selftest`), through the same analyzer
functions as the corpus. Each case is a throwaway repo copied from this
checkout, and must report rewrites > 0, change files, and build both before and
after the fix:

- go: `go/golangci/testdata/smoke` plus its SDK stubs (offline, `GOPROXY=off`);
  the fixed file must equal `flags.go.golden`.
- js: the `aliased-import` fixture input with the real npm SDK packages;
  the fixed file must equal `output.ts`.
- java: `java/smoke` with the real jars from Maven.

It needs network (or warm npm and Maven caches) and takes about 20 seconds, so
it is not part of `task check`. Java rewrites are counted from a Find-only
dryRun: together with the Migrate recipe the data table would be built from the
already rewritten tree and list no rewritable calls.

## Hardening (the corpus clones third-party code and builds it)

- Names, URLs and SHAs come only from `spec/corpus.json` and are validated
  (name `[a-z0-9-]`, `https://github.com/<owner>/<repo>`, 40-hex SHA, bounded
  timeout and depth). Every child process is spawned with an argument array, never
  through a shell, and nothing read from a cloned repo is used in a command.
- `git fetch` allows only https, and nothing in the clone (hooks, config) runs.
  npm installs use `--ignore-scripts`; Go uses `GOTOOLCHAIN=local`.
- The workflow has only `contents: read`, no secrets, no
  `pull_request` trigger (cron and `workflow_dispatch` only), no persisted
  checkout credentials, and job and per-repo timeouts.
- Not hardenable: `go build` and `mvn compile` run whatever the cloned
  project's build asks for (Maven plugins and annotation processors, cgo). The
  mitigation is the pin: a repo is only ever built at the reviewed SHA, on an
  ephemeral runner with a read-only token that is not exposed to the steps.
