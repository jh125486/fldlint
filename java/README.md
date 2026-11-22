# Java

Java support is planned for milestone M3. Nothing is implemented yet; this
directory holds documentation and draft fixtures until #9 scaffolds the build.

Plan (see [ADR 009](../docs/adr/009-java-openrewrite-recipes.md), status Proposed):

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
- Spec changes: [proposed-rules.patch.md](proposed-rules.patch.md). Evidence:
  [spike-findings.md](spike-findings.md).
