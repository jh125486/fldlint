# Java

Java support is milestone M3. The Maven build, the generated rules table
(`src/main/java/io/github/jh125486/fldlint/spec/RulesGen.java`, written by the Go
generator, drift-checked) and the JUnit conformance harness exist (#9). Issue #10
adds the first rule, `bool-variation`: the shared `Planner`, the detector, the
OpenFeature client binding proof and two recipes. Build and test with
`./mvnw -B verify` (JDK 21) or `task java:test` from the repository root.

What exists (package `io.github.jh125486.fldlint`):

- `planner.Planner.plan(CallSite) -> PlanResult`: the shared step order (ADR 010),
  pure data in and out. `Planner.recognizesArgCount` enforces `argCount`.
- `LaunchDarklyCall.detect`: match (`LDClientInterface`, overrides included),
  static key, `LDContext.create(..)` context, default by static type, receiver
  purity, comment placement. `ClientBindingProof`: the OpenFeature `Client` proof.
- `BoolVariationToOpenFeature` (rewrite) and `FindLaunchDarklyUsage` (search plus
  the `LaunchDarklyUsage` data table with position, skip, no-fix and fingerprint).
  The composite `META-INF/rewrite/fldlint.yml`
  (`MigrateLaunchDarklyToOpenFeature`) lists the rewrite recipe only; the search
  recipe is `dryRun`-only (ADR 009 D5).
- `Fingerprint`: spec/contracts/fingerprint.md, tested against the shared vectors.

The conformance harness runs every case with an `expect.java`: diagnostics from the
search recipe, the migrated output against `After.java` (or no change for a skip),
and idempotency; `FixtureCompilesTest` compiles every `Before.java` and
`After.java` with javac and the type-attribution test runs `FindMissingTypes`.
The `java-ld-user` case needs the v6 `LDUser` overload, absent from the 7.x jar, so
it is parsed and compiled against stubs in `src/test/resources/stubs/ld-v6`
instead of the LaunchDarkly jars. To regenerate `After.java` files from the
recipe: `./mvnw -B test -Dtest='ConformanceTest#fixtures' -Dfldlint.updateFixtures=true`,
then review the diff.

Not done yet: the other rules (#11), homing comments inside a call (Java reports
such a call with no fix, `[no-fix: comment]`), constants as static keys, inherited
client fields, and a CLI or plugin run (only `RewriteTest`).

Plan (see [ADR 009](../docs/adr/009-java-openrewrite-recipes.md), status Accepted):

- Imperative OpenRewrite recipes under `io.github.jh125486.fldlint`, artifact
  `io.github.jh125486:fldlint-rewrite`, built with Maven against the last Maven
  Central OpenRewrite release (`rewrite-java` 8.90.4). Use with
  `rewrite-maven-plugin` 6.46.1; current-plugin compatibility is UNVERIFIED.
- Type attribution proves a call is bound to `LDClientInterface` and that an OpenFeature
  `Client` is in scope before anything is rewritten.
- Rewrites keep primitive typing by appending `.booleanValue()`, `.intValue()` or
  `.doubleValue()` (ADR 009 D8); fixtures in [fixtures/](fixtures/).
- `FindLaunchDarklyUsage` is `dryRun`-only (`run` writes markers into source); pass
  `-DfailOnDryRunResults=true` to fail on any finding.
- Spec input: read `spec/generated/spec.json` with Jackson (test scope; `-Dfldlint.specRoot=<dir>` selects another root). Java never
  parses YAML (no SnakeYAML); the JS loader is the one YAML reader
  ([ADR 011](../docs/adr/011-parse-yaml-once-emit-canonical-json.md)).
- Spec changes: [proposed-rules.patch.md](proposed-rules.patch.md). Evidence:
  [spike-findings.md](spike-findings.md).

Argument counts: report-only rules carry `argCount` (`min`, and `max` unless variadic) on
each language binding, exposed as `Binding.getArgCountMin()` / `getArgCountMax()`
(null when absent). `Planner.recognizesArgCount` enforces them: a call whose argument
count is outside the range is an unrecognized call (no finding), as in the JS and Go
engines. No report-only rule has a Java binding today (#11), so every Java `argCount`
is currently null and the range is covered by unit tests only.
