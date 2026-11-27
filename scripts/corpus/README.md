# Real-repo corpus

Issue #21, Part 2. Runs the fldlint tools over pinned public repos that use the
LaunchDarkly server SDK, in report mode first, then applies the fixes in a temp
copy and builds it. Read-only with respect to the world: it never pushes, opens
PRs or comments upstream, copies no code into this repository (only
`url` + `sha` live in [`spec/corpus.yaml`](../../spec/corpus.yaml)) and deletes
its temp dirs.

```sh
task corpus            # all repos, compare with spec/corpus-snapshot.json
task corpus:js         # or corpus:go, corpus:java
task corpus:update     # re-run everything and rewrite spec/corpus-snapshot.json
node scripts/corpus/run.mjs run --only hello-go,ld-sample-app-go   # a subset
```

Needs network, `git`, Node (the `yaml` package from `js/` is the only import
beyond Node built-ins), Go (the repo's toolchain), and JDK 21 for Java. Results
go to `tmp/corpus/result.json`; a post-fix build failure also leaves
`tmp/corpus/<repo>.fix.patch`. Set `RUNNER_TEMP` to choose the temp root and
`CORPUS_KEEP_PATCHES=1` to keep every fix patch.

## What runs per repo

Each repo gets its own deadline (`timeout` in `spec/corpus.yaml`, seconds, for
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
