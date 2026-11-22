# eslint-plugin-fldlint

ESLint plugin that finds LaunchDarkly server SDK (`@launchdarkly/node-server-sdk`)
calls that can migrate to OpenFeature, and rewrites the ones it can prove safe.
Behaviour follows ESLint exactly ([ADR 008](../../../docs/adr/008-idiomatic-per-ecosystem.md)):
severity, `--max-warnings`, `--quiet`, `--fix`, `--fix-dry-run`, formatters and
inline suppression are all ESLint's. fldlint adds no formatter or flag.

Requires ESLint 9 (flat config) and Node 22.12 or newer. ESLint 10 is not yet supported.

## Install

> The package is `private` until the first release (#20), so it is not on the npm
> registry yet. Until then, use it from a checkout of this repository
> (`npm install --save-dev /path/to/fldlint/js/packages/eslint-plugin` after
> `npm ci && npm run build` in `js/`). The command below is the install line
> that will apply once it is published.

```sh
npm install --save-dev eslint eslint-plugin-fldlint
# for TypeScript sources
npm install --save-dev @typescript-eslint/parser
```

## Usage (flat config)

```js
// eslint.config.js
import fldlint from "eslint-plugin-fldlint";
import * as tsParser from "@typescript-eslint/parser";

export default [
  fldlint.configs.recommended, // every fldlint rule at "warn"
  { files: ["**/*.ts"], languageOptions: { parser: tsParser } },
  // per-rule severity: "off" | "warn" | "error"
  { rules: { "fldlint/all-flags-state": "off", "fldlint/bool-variation": "error" } },
];
```

The preset sets no parser (plugins cannot force one). Plain JavaScript works with
ESLint's default parser; for TypeScript supply `@typescript-eslint/parser`.

Run it like any ESLint rule set:

```sh
eslint src                 # report
eslint --fix src           # apply the provable rewrites
eslint --fix-dry-run -f json src
```

Suppress with ESLint's own comments, e.g.
`// eslint-disable-next-line fldlint/bool-variation`.

## Fixes

`meta.fixable` is `"code"`. A fix is emitted only when the flag key is a static
string, the default has the right literal type, the LaunchDarkly context is a
simple `{ key }` / `{ kind: "user", key }` literal, and an OpenFeature client
binding (`OpenFeature.getClient()` assigned to a variable in scope) is proven.
That decision is made by `@fldlint/core`'s `plan()`, the same planner the
codemod and CLI use. The legacy `launchdarkly-node-server-sdk` package is never
rewritten.

The LaunchDarkly client is left in place (an unused import or variable is
yours to remove; under `--noUnusedLocals` TS6133 may flag it). Nested rewrites
are resolved by ESLint's multipass fixing.

## Skips

A call that is detected but cannot be rewritten is still reported, **without a
fix**, so it shows up in CI. Its message is the spec's skip-category text with
`[skip: <category>]` appended, for example
`Flag key is not a static string [skip: dynamic-key]`. Categories: `detail`,
`bulk`, `type-mismatch`, `dynamic-key`, `unbound-client`, `context-unproven`,
`default-type-unproven`, `receiver-unproven`.

## Rules

One ESLint rule per rule id in `spec/rules.yaml` that has a JavaScript method
list. Go-only ids (`int-variation`, `go-json-variation`, `int-variation-detail`)
have no ESLint rule. Messages and ids come from the spec; the rule-level
`skip` category of a report-only rule is its only possible message.
Reports are positioned at the call expression (1-based line and UTF-16 column).

Rewritable (message `Migrate <type> flag <key> to OpenFeature`, else a skip):

### bool-variation
### string-variation
### number-variation
### json-variation

Report-only (always `[skip: <category>]`):

### variation
`type-mismatch`: `variation()` returns `any`; the typed target cannot be chosen.
### migration-variation
`type-mismatch`: migration flags return a stage plus a tracker.
### bool-variation-detail
### string-variation-detail
### number-variation-detail
### json-variation-detail
### variation-detail
`detail`: reason metadata has no 1:1 rewrite.
### all-flags-state
`bulk`: bulk evaluation; migrate manually.

The authoritative list is `Object.keys(plugin.rules)`.

## Gating CI

With the default `--max-warnings -1`, warnings never fail a run. Either:

- set the rules you want enforced to `"error"` (exit `1` on any finding), or
- keep `warn` and pass `--max-warnings 0`:
  `eslint --max-warnings 0 src`.

Exit codes are ESLint's: `0` clean (or warnings within the limit), `1` errors or
too many warnings, `2` configuration or internal error. `--quiet` only hides
warnings from the output; it does not change the exit code. With `--fix` the
code is computed from what remains.

### Baselines

fldlint's `fldlint-baseline.json` belongs to the standalone CLI and is never read
by this plugin. To adopt gradually, set the fldlint rules to `"error"` and use
ESLint's bulk suppressions (`eslint --suppress-all`, `--prune-suppressions`).
ESLint applies suppressions only to rules at `error`.

## Development

`npm test -w eslint-plugin-fldlint` runs the shared fixtures
(`spec/fixtures/**/js`) through ESLint's `Linter` with both
`@typescript-eslint/parser` and espree (where the fixture is plain JS),
checking diagnostics, `--fix` output and idempotence, plus the real ESLint CLI
for severity and `--max-warnings` behaviour.
