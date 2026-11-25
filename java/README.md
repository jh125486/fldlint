# Java

Java support is milestone M3. The Maven build, the generated rules table
(`src/main/java/io/github/jh125486/fldlint/spec/RulesGen.java`, written by the Go
generator, drift-checked), a stub planner and the JUnit conformance harness exist
(#9); the recipes themselves land in #10. Build and test with `./mvnw -B verify`
(JDK 21) or `task java:test` from the repository root.

Known gap for #10: the smoke fixture
`spec/fixtures/bool-variation/static-key-double-quote/java/After.java` expects
`.booleanValue()` appended (ADR 009 D8) and `LDContext.create("user")` becoming
`new ImmutableContext("user")`. The Java template in `spec/rules.yaml` does not
produce either yet; #10 must reconcile the template and the fixture.

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
(null when absent). The Java engine does not exist yet (#10), so nothing enforces
them here; the table only carries the values. No report-only rule has a Java
binding today, so every Java `argCount` is currently null. When one is added,
the recipe must treat a call whose argument count is outside the range as an
unrecognized call (no finding), as the JS and Go engines do.
