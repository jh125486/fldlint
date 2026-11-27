# README walkthroughs

Every command below was run from a checkout of this repository on 2026-11-26
(macOS arm64, Node v26.13.1, go1.27.4, Temurin JDK 21.0.13). The inputs are
shared fixtures from [`spec/fixtures/bool-variation/`](../spec/fixtures/bool-variation/);
nothing is published, so everything installs from the checkout. `<repo>` is the
path of your checkout, `<tmp>` a scratch directory outside it.

Two fixtures are used throughout:

- `aliased-import`: a call fldlint can prove and rewrite.
- `dynamic-key`: a call whose flag key is computed at runtime, so it is
  reported with `[skip: dynamic-key]` and left alone.

## JavaScript and TypeScript

### ESLint plugin

```sh
cd <repo>/js && npm ci && npm run build

mkdir -p <tmp>/jsproj/src && cd <tmp>/jsproj
echo '{"name":"demo","private":true,"type":"module"}' > package.json
npm install --save-dev eslint@10.15.0 @typescript-eslint/parser@8.74.0 <repo>/js/packages/eslint-plugin
cp <repo>/spec/fixtures/bool-variation/aliased-import/js/input.ts src/darkmode.ts
cp <repo>/spec/fixtures/bool-variation/dynamic-key/js/input.ts src/feature.ts
```

`eslint.config.js` (from the plugin README):

```js
import fldlint from "eslint-plugin-fldlint";
import * as tsParser from "@typescript-eslint/parser";

export default [
  fldlint.configs.recommended,
  { files: ["**/*.ts"], languageOptions: { parser: tsParser } },
];
```

Observed:

```text
$ npx eslint src      # paths shortened
src/darkmode.ts
  8:16  warning  Migrate boolean flag "dark-mode" to OpenFeature  fldlint/bool-variation

src/feature.ts
  8:16  warning  Flag key is not a static string [skip: dynamic-key]  fldlint/bool-variation

✖ 2 problems (0 errors, 2 warnings)
  0 errors and 1 warning potentially fixable with the `--fix` option.
(exit 0)

$ npx eslint --max-warnings 0 src      # exit 1: "ESLint found too many warnings (maximum: 0)."
$ npx eslint --fix src/darkmode.ts     # exit 0, file rewritten
```

After `--fix`, `src/darkmode.ts` equals the fixture's `js/output.ts`.

### Codemod

```sh
cd <repo>/js && npm run build -w @fldlint/codemod

# <target>: a directory with src/darkmode.ts and src/feature.ts (the two inputs
# above), committed to its own Git repository (or pass --allow-dirty)
cd <repo>
npx --yes codemod@1.18.7 workflow run -w js/packages/codemod -t <target> --no-interactive
```

Observed (exit 0; the codemod exits 0 even when every call is skipped, see the
[codemod README](../js/packages/codemod/README.md)):

```text
Workflow started f0ad4ab6-8656-4ac9-bca1-49ad4bb9a3e9
⏺ Rewrite TypeScript and JavaScript

src/feature.ts
  src/feature.ts:8:16  bool-variation  Flag key is not a static string [skip: dynamic-key]
⏺ Rewrite TSX and JSX
Workflow completed in 3.0s
```

`git diff` in the target then shows only `src/darkmode.ts` changed, with the
same rewrite as the ESLint fixer.

## Go

The fixtures compile against the SDK stubs in `go/analyzer/testdata/src` (a
GOPATH tree), not the real SDKs.

```sh
W=<tmp>/gow; mkdir -p $W/src/flags $W/src/skips
cp -r <repo>/go/analyzer/testdata/src/github.com $W/src/
cp <repo>/spec/fixtures/bool-variation/aliased-import/go/input.go $W/src/flags/
cp <repo>/spec/fixtures/bool-variation/dynamic-key/go/input.go $W/src/skips/
# the build runs before GOPATH is exported, on purpose (module mode)
(cd <repo>/go && go build -o $W/bin/fldlint ./cmd/fldlint)
cd $W/src
export GOPATH=$W GO111MODULE=off GOWORK=off
```

Observed (paths shortened):

```text
$ $W/bin/fldlint ./flags ./skips
flags/input.go:21:11: Migrate boolean flag "dark-mode" to OpenFeature
skips/input.go:14:11: Flag key is not a static string [skip: dynamic-key]
(exit 3)

$ $W/bin/fldlint -fix -diff ./flags      # exit 0, file untouched
$ $W/bin/fldlint -fix ./flags            # exit 0, file rewritten
```

After `-fix`, `flags/input.go` equals the fixture's `go/output.go.golden`
(checked with `diff`). The diff printed by `-fix -diff` (the `---`/`+++` header paths omitted):

```diff
@@ -1,10 +1,10 @@
 package flags
 
 import (
+	"context"
 	"fmt"
 	"time"
 
-	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
 	ld "github.com/launchdarkly/go-server-sdk/v7"
 	"github.com/open-feature/go-sdk/openfeature"
 )
@@ -18,6 +18,6 @@
 		return false
 	}
 	defer ldClient.Close()
-	on, _ := ldClient.BoolVariation("dark-mode", ldcontext.New("user"), false)
+	on, _ := client.BooleanValue(context.TODO(), "dark-mode", false, openfeature.NewEvaluationContext("user", nil))
 	return on
 }
```

## Java

```sh
cd <repo>/java && ./mvnw -B -DskipTests install       # local snapshot of the recipes
cd smoke && ../mvnw -B \
  org.openrewrite.maven:rewrite-maven-plugin:6.46.1:dryRun \
  -Drewrite.recipeArtifactCoordinates=io.github.jh125486:fldlint-rewrite:0.1.0-SNAPSHOT \
  -Drewrite.activeRecipes=io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature,io.github.jh125486.fldlint.FindLaunchDarklyUsage
```

Observed: exit 0 and `BUILD SUCCESS`. A dry run exits 0 even when it would
change files. The end of the output lists the recipes that would change files
and the patch location:

```text
[WARNING] These recipes would make changes to java/smoke/src/main/java/smoke/SmokeFlags.java:
[WARNING]     io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature
[WARNING]         io.github.jh125486.fldlint.BoolVariationToOpenFeature
[WARNING]         io.github.jh125486.fldlint.StringVariationToOpenFeature
[WARNING]         io.github.jh125486.fldlint.NumberVariationToOpenFeature
[WARNING]         io.github.jh125486.fldlint.IntVariationToOpenFeature
[WARNING]         io.github.jh125486.fldlint.FindLaunchDarklyUsage
[WARNING] Patch file available:
[WARNING]     .../java/smoke/target/rewrite/rewrite.patch
[WARNING] Run 'mvn rewrite:run' to apply the recipes.
[INFO] BUILD SUCCESS
```

With `-DfailOnDryRunResults=true` added, the same command ends in
`BUILD FAILURE` ("Applying recipes would make changes") and exits 1.

The generated `rewrite.patch`, minus its `index` lines, is identical to the
committed [`java/smoke/expected.patch`](../java/smoke/expected.patch) (checked
with `diff`). The input is
[`java/smoke/src/main/java/smoke/SmokeFlags.java`](../java/smoke/src/main/java/smoke/SmokeFlags.java).

The shared `aliased-import` fixture's Java before/after
(`spec/fixtures/bool-variation/aliased-import/java/{Before,After}.java`) is run by the
conformance test: `cd java && ./mvnw -B test -Dtest='ConformanceTest'` passes.
