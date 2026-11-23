# Java spike findings (appendix to ADR 009)

Run 2026-11-22 on JDK 21.0.13 (Temurin) and Maven 3.10.1. Scratch projects were not
committed. Every claim is **VERIFIED (how)** or **UNVERIFIED**.

## F1. OpenRewrite left Maven Central; Central is frozen
- VERIFIED (`curl` of `maven-metadata.xml`): Central's newest `rewrite-java` and
  `rewrite-test` are 8.90.4 (2026-08-24), `rewrite-maven-plugin` 6.46.1,
  `rewrite-recipe-bom` 3.38.0, `rewrite-templating` 1.44.0. `rewrite-java` 8.113.2
  returns 404 on Central.
- VERIFIED (GitHub releases API): current `rewrite` 8.113.2 (2026-11-20),
  `rewrite-maven-plugin` 6.71.0, `rewrite-templating` 1.66.0, `rewrite-recipe-bom`
  3.62.0, `rewrite-gradle-plugin` 7.44.1 (2026-10-15).
- VERIFIED (`curl` of the Gradle Plugin Portal metadata): the Portal's latest
  `org.openrewrite.rewrite` plugin is 7.41.0 (2026-08-26).
- VERIFIED (`curl -I`): `artifacts.codegenomeproject.org/maven/...` answers `401`
  with `WWW-Authenticate: Basic realm="Code Genome Project"`.
- VERIFIED (rewrite-maven-plugin README on `main`): new releases go only to the Code
  Genome Project (CGP); users add a `codegenome` repository and a `<server>` with
  credentials. UNVERIFIED: how a token is obtained and CGP terms for OSS CI use (no
  account created).
- VERIFIED (`pom.xml` of rewrite-maven-plugin 6.46.1): pins core 8.89.0.
- VERIFIED (ran it): plugin 6.46.1 plus recipe jars run with no credentials. Only the
  OpenRewrite dependencies came from Central; the spike recipe jar itself came from
  the local `~/.m2` (it was never published).

## F2. A recipe compiled against 8.90.4 runs under the Central plugin
- VERIFIED (ran it): a jar compiled against `rewrite-java` 8.90.4 with
  `maven.compiler.release=8` ran in `rewrite-maven-plugin:6.46.1:dryRun` (core 8.89.0)
  as a search recipe and as a rewriting recipe, producing the expected `rewrite.patch`.
- UNVERIFIED: the same jar under plugin 6.71.0 / core 8.113.x (needs CGP credentials).

## F3. Named typed placeholders and the template mechanics
- VERIFIED (`RewriteTest` 8.90.4; plugin run for boolean only):
  `#{client:any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key:any(java.lang.String)}, #{default:any(boolean)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})`
  rewrote a `boolVariation` call. The `of` client came from walking the enclosing
  method's parameters. The spike output (`getBooleanValue("k", false, LDContext.create("u"))`)
  intentionally does **not compile**: an `LDContext` is passed as `EvaluationContext`
  because context conversion was out of the spike (ADR 006).
- VERIFIED (`RewriteTest`, 2026-11-22 re-run): the same mechanism with `any(int)`,
  `any(double)` and `any(java.lang.String)` works, including the appended
  `.intValue()` / `.doubleValue()` / `.booleanValue()` (see ADR 009 D8 and
  `java/fixtures/primitive-typing/`). Not exercised in the plugin, only `RewriteTest`.
- VERIFIED: arguments bind by order of appearance in the template, not by name. LD
  order is (key, context, default), OpenFeature is (key, default, context).
- UNVERIFIED: one placeholder name used twice in a template.
- The placeholder type is not checked against the argument; the recipe must prove types.

## F4. Template parsing does not see the project classpath in the plugin
- VERIFIED (ran it): `JavaParser.fromJavaVersion().classpath("sdk")` works in
  `RewriteTest` and fails in the plugin (`Unable to find runtime dependencies
  beginning with: 'sdk'`). `.dependsOn("<stub source>")` works in both. Keep stubs
  minimal and test them against the real `dev.openfeature:sdk` jar.

## F5. Search recipes, `dryRun`, `run`
- VERIFIED (ran it, plugin 6.46.1): `SearchResult.found` makes `dryRun` write a patch;
  `-DfailOnDryRunResults=true` fails the build. `-Drewrite.failOnDryRunResults=true` is
  silently ignored. `rewrite:run` writes the marker comment into source, so the search
  recipe is `dryRun`-only.
- VERIFIED: `-Drewrite.exportDatatables=true` writes `SearchResults.csv`.
  UNVERIFIED: a custom `DataTable` with our columns.

## F6. Mapping checked against real artifacts (`javap` on Central jars)
- `launchdarkly-java-server-sdk` 7.18.1: `LDClientInterface` has `boolVariation`,
  `intVariation`, `doubleVariation`, `stringVariation`, `jsonValueVariation`, five
  `*Detail` variants, `migrationVariation`, `allFlagsState`; all
  `(String, LDContext, T)` returning primitives. `LDClient` is `final` and redeclares
  them, hence `matchOverrides = true`. 5.10.9 has only `LDUser` overloads, 6.3.0 both,
  7.0.0 only `LDContext`.
- `dev.openfeature:sdk` 1.23.0 `Features`: `getBooleanValue`, `getStringValue`,
  `getIntegerValue`, `getDoubleValue`, `getObjectValue`, each `(String, T,
  EvaluationContext)` with **boxed** `T` and boxed returns.
- VERIFIED (`javac`): boxing does not widen; `getDoubleValue("k", 1, e)`,
  `getIntegerValue("k", (short) s, e)` and `getIntegerValue("k", 'a', e)` do not compile.
- VERIFIED (read `dev.openfeature:sdk:1.23.0` sources jar, `OpenFeatureClient`):
  `getXValue(k, d, ctx)` returns `getXDetails(..).getValue()`. On an error code or any
  exception, `enrichDetailsWithErrorDefaults` sets the value to the caller's default.
  UNVERIFIED: third-party providers returning a `null` value on a non-error path (not
  read; the SDK does not guard it).
- D7: JDK 21 is required because the test libraries (`rewrite-java-21`) and the Maven
  3.10 toolchain need it, while the jar targets release 8 for consumers on old JDKs.
  Issue #8 mentioned JDK 25; nothing here needs more than 21, and 21 is the oldest
  current LTS. VERIFIED: JDK 21 + `release=8` compiles and loads in the plugin.
- VERIFIED: pinning `junit-jupiter` 5.11.4 beside `rewrite-test`'s transitive JUnit
  5.14.4 / platform 1.14.4 crashes surefire (`NoClassDefFoundError:
  OutputDirectoryCreator`); align versions.
