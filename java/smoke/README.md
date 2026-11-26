# Compatibility smoke project

A tiny Maven project that runs the published-style `fldlint-rewrite` jar through the
current `rewrite-maven-plugin` (ADR 009 D1 risk). It is not a module of `java/pom.xml`
and `task java:test` does not build it; the CI job of #12 runs it (`task java:smoke`, which reads the version from `java/pom.xml` and passes it as `-Dfldlint.version`).

```sh
cd java && ./mvnw -B -DskipTests install          # a local snapshot of the recipes
cd smoke && ../mvnw -B \
  org.openrewrite.maven:rewrite-maven-plugin:6.46.1:dryRun \
  -Drewrite.recipeArtifactCoordinates=io.github.jh125486:fldlint-rewrite:0.1.0-SNAPSHOT \
  -Drewrite.activeRecipes=io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature,io.github.jh125486.fldlint.FindLaunchDarklyUsage
```

Expected (checked against plugin 6.46.1): `BUILD SUCCESS` and `rewrite.patch` (under `target/rewrite/` of the directory Maven is run from, `java/smoke/` for the commands above) for
`SmokeFlags.java` with

- the four calls in `rewritten` (bool, string, double, int) replaced by
  `client.getBooleanValue(..).booleanValue()`, `getStringValue`, `getDoubleValue(..).doubleValue()`
  and `getIntegerValue(..).intValue()`;
- a `/*~~(.. [skip: <category>])~~>*/` marker (from `FindLaunchDarklyUsage`) before each call in
  `reported`: `default-type-unproven`, `dynamic-key`, `detail`, `type-mismatch` (json),
  `type-mismatch` (migration) and `bulk`, and no change to those calls;
- nothing at all for `Toggles.boolVariation` in `unrelated`.

Run only `MigrateLaunchDarklyToOpenFeature` to get the rewrites without the markers, and add
`-DfailOnDryRunResults=true` to fail the build when anything is found. The project compiles as
it is (`../mvnw -B compile`).
