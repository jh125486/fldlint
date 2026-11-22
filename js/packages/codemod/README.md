# @fldlint/codemod

Planned: an `npx codemod` package applying the same rewrites as `@fldlint/core`
(issue #15). Not yet built; this file records the platform research (spike run
2026-11-22 with `codemod@1.18.7`).

## How this works / limits

Labels: VERIFIED = observed by running it or read in the docs; UNVERIFIED = not checked.

- VERIFIED: a JSSG transform is a module whose default export receives an
  `SgRoot` and returns a string (new source) or `null` (no change). The tree it
  hands you is ast-grep/tree-sitter (oxc for semantic analysis), not ESTree, and
  has no ESLint `ScopeManager`.
- VERIFIED: the runtime is QuickJS plus LLRT, deny-by-default. `fs` is sandboxed
  to the target directory; `child_process` and `fetch` need opt-in
  `capabilities` in `codemod.yaml` (or `--allow-child-process`).
- VERIFIED: `npx codemod jssg test -l typescript <transform> <tests-dir>`; each
  case is a directory holding `input.<ext>` and `expected.<ext>`. Exit codes of
  the test runner: 0 pass, 1 failure, 2 error.
- VERIFIED: bare `node_modules` imports from an unbundled transform fail at
  runtime (`Could not find export 'lstatSync' in module 'fs'`). The module must
  be bundled to a single file.
- VERIFIED: core's `analyze()`/`rewrite()` CAN run inside the runtime if bundled
  (parse with `@typescript-eslint/parser`, binding proof via its scope manager),
  and the shared fixture `bool-variation/static-key-double-quote` passed through
  `jssg test`. So core is reusable as-is and no second engine is needed. The
  bundle needs these shims, because LLRT lacks the real modules:
  - `eslint` and `jiti` stubbed (pulled in only by `@typescript-eslint/utils`
    runtime exports that are never executed);
  - `debug` stubbed (it calls `tty.isatty` at load);
  - `node:inspector` stubbed (absent from LLRT);
  - bare `require("fs")` style calls rewritten to `node:fs`.
  The result is about 11.7 MB (it contains the TypeScript compiler).
- UNVERIFIED: bundle load time and memory on large repos (one JSSG instance per
  file); `console.log` reaching the terminal from `workflow run`; exit codes of
  `workflow run` / `jssg run`; publishing via `codemod/publish-action`.

Blocker for the build: bundling needs a bundler (esbuild or rolldown, the latter
is only a transitive dependency of vitest). Neither is on the approved
dependency list in CONTRIBUTING.md, so it needs an owner decision first.
