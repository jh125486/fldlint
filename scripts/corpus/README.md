# Real-repo corpus

Issue #21, Part 2. Runs the fldlint tools over pinned public repos that use the
LaunchDarkly server SDK, in report mode first, then applies the fixes in a temp
copy and builds it. It never pushes, opens PRs or comments upstream, copies no
code into this repository (only `url` + `sha` live in
[`spec/corpus.json`](../../spec/corpus.json)) and deletes its temp dirs (also
on SIGINT and SIGTERM).

```sh
task corpus            # all repos, compare with spec/corpus-snapshot.json
task corpus:js         # or corpus:go, corpus:java
task corpus -- --only hello-go,ld-sample-app-go   # a subset (also after corpus:go etc.)
task corpus:test       # unit tests of the pass/fail logic and the env allowlist (offline)
task corpus:selftest   # apply-fix-then-build on local fixtures (see below)
task corpus:update     # re-run everything and rewrite spec/corpus-snapshot.json
```

`task corpus:go` includes `weaviate`, a roughly 1 GB repository; use
`task corpus:go -- --only hello-go,ld-sample-app-go` for a quick run.

Needs network, `git`, Node (built-ins only; the corpus is JSON, and `npm ci` in
`js/` is needed only to build the ESLint plugin for the JS repos), the Go
toolchain on `PATH` (run with `GOTOOLCHAIN=local`, so a repo that needs a newer
Go than the installed one fails its build instead of downloading a toolchain)
and JDK 21 for Java. Results go to `tmp/corpus/result.json`, rewritten after
every repo; a post-fix build failure also leaves `tmp/corpus/<repo>.fix.patch`.
Set `RUNNER_TEMP` to choose the temp root and `CORPUS_KEEP_PATCHES=1` to keep
every fix patch.

## What runs per repo

Each repo gets its own deadline (`timeout` in `spec/corpus.json`, seconds, for
clone, dependencies, tools and both builds; the process group is killed at the
deadline and the repo is reported as `timeout`). The clone is
`git fetch --depth <depth> origin <sha>` of exactly the pinned commit.

| Lang | Report | Fix (temp copy) | Build before and after the fix |
|------|--------|-----------------|--------------------------------|
| js | ESLint flat config with the plugin built from this checkout and `@typescript-eslint/parser`, `--format json` ([walkthroughs](../../docs/walkthroughs.md)) | `eslint --fix` | `tsc --noEmit` when a `tsconfig.json` exists (after `npm install --ignore-scripts`) |
| go | `fldlint -json ./...` built from `go/`, run in the repo's own module (`go mod download`, or `go mod tidy` when no `go.sum` is committed) | `fldlint -fix ./...` | `go build ./...` |
| java | `rewrite-maven-plugin:dryRun` with the `FindLaunchDarklyUsage` recipe of [`java/smoke`](../../java/smoke/README.md) and `-Drewrite.exportDatatables=true` (the `LaunchDarklyUsage` table) | `rewrite:run` with `MigrateLaunchDarklyToOpenFeature` only | `mvn -q compile` |

Dependency prep (npm install, go mod tidy) is committed in the temp repo before
the fix, and that commit must succeed, so the fix diff contains only fldlint's
edits.

Repos without a committed `go.sum` are resolved at run time by `go mod tidy`
(recorded as `tidied: true`): their dependency versions are the newest that
satisfy `go.mod` on the day of the run, so for those repos the SHA pin does not
fully pin the build and a result can change without a code change.

## Output

Per repo: `status` (`ok`, `error`, `timeout`), `errorKind` (`infra` or `tool`,
see below), `counts` (`total`; `rewrites` by rule; `skips` by skip category;
`noFix` by `rule:category`), `fixedFiles`, `build.pre` and `build.post` (`ok`,
`exit`, `log` tail, or `skipped` with a reason) and `durationMs`. The committed
`spec/corpus-snapshot.json` is the same minus the volatile fields (times, logs).
Nothing read from a cloned repo's data files is echoed into the result except
build and tool log tails.

A finding is a **rewrite** when it carries neither `[skip: ...]` nor
`[no-fix: ...]`. Rewrites only happen where the OpenFeature client binding is
provable in scope, so a repo that has never adopted OpenFeature reports skips
only; zero rewrites is normal for most of the corpus.

## Pass / fail

`run.mjs compare` (logic in `compare.mjs`, run by `task corpus`
and the weekly workflow; unit-tested in `compare.test.mjs`) **fails** when:

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
   under `spec/fixtures/`, fix the engine, then re-run.
3. **A tool error.** A repo ends with `status: error` and `errorKind: tool`
   (fldlint, ESLint or the rewrite plugin crashed, printed no JSON, exported no
   data table, or the analysis reported package errors for a repo that builds).
4. **Nothing was verified**: no repo has `status: ok`, or the result is empty.
5. **Findings vanish**: `counts.total` drops from above zero to zero, or a
   `noFix` count rises (a rewrite was withheld).

It only **warns** (exit 0) on infrastructure trouble: a clone, fetch,
dependency step or timeout (`errorKind: infra`, `timeout`); a build that was
already broken before the fix (no safety verdict); a post-fix build that timed
out. Everything else is a `note:`: a rewrite count that falls, changed skip
counts, a changed pinned SHA, files that failed to parse.

## Self-test

No corpus repo produces a rewrite (none has an OpenFeature client in scope), so
the apply-fix-then-build path is exercised separately by
`task corpus:selftest` (`run.mjs selftest`), through the same analyzer
functions as the corpus. Each case is a throwaway repo copied from this
checkout, and must report rewrites > 0, change files, and build both before and
after the fix:

- go: `go/golangci/testdata/smoke` plus its SDK stubs (offline, `GOPROXY=off`);
  the fixed file must equal `flags.go.golden`. This case, about 2 seconds, is
  part of `task check` (as `corpus:selftest:go` in `check:go`), together with
  `corpus:test` in `check:shared`.
- js: the `aliased-import` fixture input with the real npm SDK packages; the
  fixed file must equal `output.ts`. It first deletes every workspace `dist/`
  and rebuilds `js/` from scratch (so a build that only works from stale
  output, for example with lifecycle scripts disabled, is caught), and plants a
  hostile `.npmrc` registry that the pinned environment must override.
- java: `java/smoke` with the real jars from Maven.

The JS and Java cases need the network (or warm npm and Maven caches) and take
about 40 seconds, so they are not part of `task check`. Java rewrites are
counted from a Find-only dryRun: together with the Migrate recipe the data
table would be built from the already rewritten tree and list no rewritable
calls.

## Security: this runs third-party build tooling

Building a pinned third-party repository executes its build tooling, and no
setting here makes that safe by itself: `tsc` is taken from the repo's own
`node_modules/.bin` when it has one, Maven honours the repo's
`.mvn/jvm.config`, `.mvn/maven.config` and `.mvn/extensions.xml`, its plugins
and annotation processors run during `mvn compile`, `go build` runs cgo and
whatever the toolchain compiles, and npm reads the repo's `.npmrc` for
everything the environment does not override. What limits the damage:

- **The SHA pin.** A repo is built only at the reviewed commit in
  `spec/corpus.json` (but see the `go.sum` note above for floating
  dependencies).
- **An ephemeral runner** with `permissions: contents: read`, no secrets, no
  `pull_request` or `pull_request_target` trigger (cron and `workflow_dispatch`
  only), `persist-credentials: false`, job and per-repo timeouts, and no
  dependency caches restored or saved (nothing a third-party build writes can
  be reused by another workflow). Artifacts are only `result.json` and fix
  patches.
- **An allowlisted child environment** (`childEnv` in `run.mjs`): `PATH`,
  `HOME`, locale, temp dirs, `JAVA_HOME`, `GOROOT`/`GOPATH`, proxy and CA
  variables. `GH_TOKEN`, `AWS_*`, `SSH_AUTH_SOCK`, npm tokens and the rest never
  reach a child, including this checkout's own trusted builds.
- **Pinned tool config for third-party steps**: git runs with
  `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`,
  `core.hooksPath=/dev/null` and https-only; npm gets
  `NPM_CONFIG_REGISTRY=https://registry.npmjs.org/`, `IGNORE_SCRIPTS=true`,
  empty user and global config and a non-existent git binary (environment beats
  a project `.npmrc`; the self-test checks it); Go runs with `GOENV=off`,
  `GOTOOLCHAIN=local` and a per-run `GOMODCACHE` and `GOCACHE`; Maven uses a
  per-run `-Dmaven.repo.local`; the npm cache is per run too.
- **Validated inputs.** Names, URLs and SHAs come only from `spec/corpus.json`
  and are validated (name `[a-z0-9-]`, `https://github.com/<owner>/<repo>`,
  40-hex SHA, bounded timeout and depth, string types). Every child process is
  spawned with an argument array, never through a shell, and no string read from
  a cloned repo is interpolated into a command line. Result files are read only
  if they are regular files (no symlinks).
