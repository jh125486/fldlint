# @fldlint/codemod

An [`npx codemod`](https://codemod.com) package that rewrites proven LaunchDarkly
server SDK calls to OpenFeature. It runs `@fldlint/core`'s own parse, detect,
plan and apply steps, so it makes exactly the rewrites the ESLint plugin's
fixer makes. Nothing is re-implemented here.

## Usage

From a checkout (registry publishing is tracked in #20):

```sh
cd js && npm ci && npm run build -w @fldlint/codemod
npx codemod@1.18.7 workflow run -w js/packages/codemod -t path/to/your/project --no-interactive
```

Once published: `npx codemod @fldlint/launchdarkly-to-openfeature`.

- `--dry-run` previews; `--allow-dirty` is needed outside a clean Git tree.
- JS and TS files are processed (`.ts .mts .cts .js .mjs .cjs .tsx .jsx`); `node_modules`, `dist` and `.d.ts` are skipped.
- The legacy `launchdarkly-node-server-sdk` package is never rewritten (core rule).

## Safety model

- A call is rewritten only if core proves, with the TypeScript-ESLint scope
  manager, that the receiver is a LaunchDarkly client and an OpenFeature client
  binding is visible at the call. Anything unproven is left untouched. A file
  is never partly rewritten at a skipped site; skip categories and the step
  order (ADR 010) come from core's `plan()`.
- Nested rewrites are applied by repeating the pass until nothing changes (at most
  10 passes, like `eslint --fix`), so a second run changes nothing (idempotent).
- A file that fails to parse is left untouched and reported.

## Skip reporting

Each skipped site and each unparseable file is printed to stdout, one line per
finding, in the shared text shape of `spec/contracts/output-format.md`:

```
src/c.ts:9:20  bool-variation  OpenFeature client binding not proven in scope [skip: unbound-client]
src/bad.ts  parse-error  could not analyze file, left unchanged: Variable declaration expected.
```

Paths are relative to the target directory. Codemod groups lines by file and
the file order between files is not deterministic. Positions refer to the
original file. Skips are reported on the first pass only.

## How this works / limits

Labels: VERIFIED = observed by running `codemod@1.18.7`; UNVERIFIED = not checked.

- VERIFIED: a JSSG transform is `export default (sg, options) => string | null`
  running in QuickJS + LLRT. The tree it provides is ast-grep, not ESTree, with
  no ESLint scope manager. We do not use that tree. The transform takes the file
  text and calls core, which parses it again with `@typescript-eslint/parser`.
  So core is reused as-is and the binding proof is unchanged.
- VERIFIED: an unbundled transform cannot import `node_modules`. `scripts/build.mjs`
  bundles `src/transform.ts` with esbuild (devDependency, build-time only) into
  one file, `dist/transform.js` (about 11.7 MB, because it contains the TypeScript
  compiler). It is not committed; build it before running or publishing.
- The build applies four runtime shims (all for code never executed on our path):
  `eslint`/`jiti` stubbed, `debug` stubbed (calls `tty.isatty` at load),
  `node:inspector` stubbed, CJS `require("fs")` style calls routed to `node:fs`.
- The bundle only runs inside Codemod's runtime and vitest, not as plain Node ESM
  (`__filename` is undefined there).
- Reproducibility: `npm run build:check -w @fldlint/codemod` rebuilds and fails
  if `dist/transform.js` differs from a fresh build or contains an absolute path.
  Proposed CI step: `cd js && npm ci && npm run build:check -w @fldlint/codemod`.
  Wiring it into `.github/workflows` is left to the orchestrator.
- Tests: `npm test -w @fldlint/codemod` runs every `spec/fixtures/**/js` fixture
  through the built bundle (output equals `output.ts`, idempotent, `null` when
  unchanged, skip lines match the case's diagnostics). `npm run test:jssg -w
  @fldlint/codemod` generates `jssg test` case dirs from the same fixtures and
  runs them in the real runtime (needs network for `npx codemod`).
- VERIFIED observed exit codes (`codemod@1.18.7`, unchanged by our transform):
  `workflow run` and `jssg run` exit 0 with changes, with no changes, with skips,
  and with an unparseable file, and also for a missing `-t` target (so CI cannot
  detect skips from the exit code; read the output). `workflow run` exits 1 on a
  non-Git or dirty target without `--allow-dirty`. `jssg run` exits 2 on a usage
  error. `jssg test` exits 0 pass, 1 failure, 2 error.
- VERIFIED: `console.log` from the transform reaches the terminal under
  `workflow run` and `jssg test`.
- UNVERIFIED: publishing via `codemod/publish-action`; behaviour on very large
  repos (about 1.4 s for a 4-file run including bundle load).
