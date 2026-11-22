# Java

Java support is planned for milestone M3. Nothing is implemented yet; this
directory holds documentation only until #9 scaffolds the build.

Plan (see [ADR 009](../docs/adr/009-java-openrewrite-recipes.md)):

- Imperative OpenRewrite recipes under `io.github.jh125486.fldlint`, artifact
  `io.github.jh125486:fldlint-rewrite`, built with Maven against the last
  Maven Central OpenRewrite release (`rewrite-java` 8.90.4); no Code Genome
  Project credentials are needed to build or to use it with
  `rewrite-maven-plugin` 6.46.1.
- Type attribution proves a call is bound to the LaunchDarkly
  `LDClientInterface` and that an OpenFeature `Client` is in scope before
  anything is rewritten.
- `FindLaunchDarklyUsage` is a search recipe: run it with `dryRun` only
  (`run` writes marker comments into source). Pass `-DfailOnDryRunResults=true`
  to fail on any finding.
- The Java rule mapping is verified against the published jars; see
  [proposed-rules.patch.md](proposed-rules.patch.md) (to be merged into
  `spec/rules.yaml` by #7).
