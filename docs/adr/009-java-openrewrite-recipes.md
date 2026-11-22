# ADR 009: Java recipes are imperative OpenRewrite recipes built on frozen Maven Central versions

Date: 2026-11
Status: Accepted

Every claim below is labelled **VERIFIED (how)** or **UNVERIFIED**. The spike was
run on 2026-11-22 with JDK 21.0.13 (Temurin) and Maven 3.10.1; scratch projects
were not committed.

## Context

ADR 002 fixes "one engine per language, shared data" and the owner has decided
that OpenRewrite stays the Java tool and that the groupId is
`io.github.jh125486`. ADR 008 says the Java tool follows the Maven/Gradle
rewrite plugin conventions, so this ADR adds no exit codes or flags. This ADR
decides how the recipes are built (issue #8) and settles the open questions
that block #9-#12 and the Java rules in #7.

## Findings (spike)

**F1. OpenRewrite left Maven Central; Central is frozen.**
- VERIFIED (`curl` of `maven-metadata.xml`, 2026-11-22): Central's newest
  `rewrite-java` and `rewrite-test` are 8.90.4 (2026-08-24),
  `rewrite-maven-plugin` 6.46.1 (2026-08-12), `rewrite-recipe-bom` 3.38.0,
  `rewrite-templating` 1.44.0. `rewrite-java` 8.113.2 returns 404 on Central.
- VERIFIED (GitHub releases API): current versions are `rewrite` 8.113.2
  (2026-11-20), `rewrite-maven-plugin` 6.71.0 (2026-11-20), `rewrite-templating`
  1.66.0. UNVERIFIED: BOM 3.62.0 and Gradle plugin 7.44.1 (quoted from the
  Moderne docs page, not re-checked).
- VERIFIED (`curl -I`): `https://artifacts.codegenomeproject.org/maven/...`
  answers `401` with `WWW-Authenticate: Basic realm="Code Genome Project"`.
- VERIFIED (read rewrite-maven-plugin README on `main`, "Artifact repository"):
  new releases are published only to the Code Genome Project (CGP); users add a
  `codegenome` `<repository>` and `<pluginRepository>` plus a `<server>` with
  credentials in `settings.xml`. Moderne's CGP page says OSS recipes are
  available "to any authenticated user" with HTTP Basic auth and a token.
  UNVERIFIED: how a token is obtained, and CGP terms for OSS CI use. I did not
  create an account.
- VERIFIED (`pom.xml` of rewrite-maven-plugin 6.46.1): it pins core
  `rewrite.version` 8.89.0.
- VERIFIED (ran it): a user can use plugin 6.46.1 and recipe artifacts entirely
  from Central, with no credentials. The recipe jar goes in the plugin's
  `<dependencies>`.

**F2. A recipe compiled against 8.90.4 runs under the Central plugin.**
- VERIFIED (ran it): a recipe jar compiled against `rewrite-java` 8.90.4 with
  `maven.compiler.release=8` on JDK 21 ran in `rewrite-maven-plugin:6.46.1:dryRun`
  (core 8.89.0, older than the compile-time version) both as a search recipe
  and as a rewriting recipe, producing the expected `rewrite.patch`.
- UNVERIFIED: the same jar under current plugin 6.71.0 / core 8.113.x. That
  needs CGP credentials, which I did not have. The recipes use only long-stable
  API (`MethodMatcher`, `JavaTemplate`, `JavaIsoVisitor`, `SearchResult`,
  `TypeUtils`), but this is not evidence.

**F3. Named typed placeholders with primitive types work.**
- VERIFIED (`RewriteTest` on `rewrite-test` 8.90.4, and end to end in the
  plugin): `JavaTemplate.builder("#{client:any(dev.openfeature.sdk.Client)}.getBooleanValue(#{key:any(java.lang.String)}, #{default:any(boolean)}, #{context:any(dev.openfeature.sdk.EvaluationContext)})")`
  rewrote `ldClient.boolVariation("k", LDContext.create("u"), false)` to
  `of.getBooleanValue("k", false, LDContext.create("u"))`, where `of` was the
  `dev.openfeature.sdk.Client` parameter found by walking the enclosing method.
- VERIFIED: template arguments bind by **order of appearance** in the template,
  not by name. The LD call order is (key, context, default); the OpenFeature
  order is (key, default, context), so the generator must emit arguments in
  template order.
- UNVERIFIED: reuse of one placeholder name twice in a template (not needed).
- The placeholder type is **not checked** against the argument (a spike passed
  an `LDClient` for `any(Client)` and it parsed). The recipe must prove types
  itself; the template never will.

**F4. Template parsing does not see the project classpath in the plugin.**
- VERIFIED (ran it): `JavaParser.fromJavaVersion().classpath("sdk")` works in
  `RewriteTest` but fails in the plugin with
  `Unable to find runtime dependencies beginning with: 'sdk'` because the
  plugin's runtime classpath does not contain the user's OpenFeature jar.
- VERIFIED: replacing it with `.dependsOn("package dev.openfeature.sdk; public interface Client {...}", ...)`
  (stub sources) works in both. So stubs are required, not optional. Keep the
  stubs minimal and covered by a test that compiles them against the real
  `dev.openfeature:sdk` jar.

**F5. A search-only recipe fails `failOnDryRunResults` and `run` rewrites source.**
- VERIFIED (ran it, plugin 6.46.1): `SearchResult.found(...)` makes `dryRun`
  report "These recipes would make changes" and write a patch. With
  `-DfailOnDryRunResults=true` the build fails
  (`Applying recipes would make changes`). The user property is
  `failOnDryRunResults`; `-Drewrite.failOnDryRunResults=true` is silently
  ignored (BUILD SUCCESS). Source: `@Parameter(property = "failOnDryRunResults")`
  in `AbstractRewriteDryRunMojo` v6.46.1.
- VERIFIED: `rewrite:run` with the search recipe **writes the marker comment**
  (`/*~~(LD boolVariation)~~>*/`) into the source file. The search recipe must
  be documented and wired as `dryRun`-only.
- VERIFIED: `-Drewrite.exportDatatables=true` writes
  `target/rewrite/datatables/<timestamp>/org.openrewrite.table.SearchResults.csv`
  with the marker text per finding. UNVERIFIED: a custom `DataTable` with our
  columns (not built in the spike).

**F6. Mapping verified against real artifacts** (`javap` on the Central jars,
2026-11-22; details in `java/proposed-rules.patch.md`).
- `com.launchdarkly:launchdarkly-java-server-sdk` 7.18.1:
  `LDClientInterface` has `boolVariation`, `intVariation`, `doubleVariation`,
  `stringVariation`, `jsonValueVariation` (`LDValue`), the five `*Detail`
  variants (`EvaluationDetail<T>` in `com.launchdarkly.sdk`),
  `migrationVariation`, `allFlagsState`; all `(String, LDContext, T)`.
  `LDClient` is `final` and declares its own `boolVariation`, hence
  `matchOverrides = true`. 5.10.9 has only `LDUser` overloads, 6.3.0 has both
  (`LDUser` ones are `default`), 7.0.0 has only `LDContext`. Package is
  `com.launchdarkly.sdk.server` in all three.
- `dev.openfeature:sdk` 1.23.0: `Features` has `getBooleanValue`,
  `getStringValue`, `getIntegerValue`, `getDoubleValue`, `getObjectValue`, each
  `(String, T, EvaluationContext)` with **boxed** `T` (`Boolean`, `String`,
  `Integer`, `Double`, `Value`). `ImmutableContext(String)` exists.
- **New constraint** (VERIFIED with `javac`): boxing does not widen, so
  `getDoubleValue("k", 1, e)` (int literal), `getIntegerValue("k", s, e)`
  (`short`) and `getIntegerValue("k", 'a', e)` (`char`) do not compile. LD's
  primitive signatures accept them. The recipe may only rewrite when the
  default argument's type is exactly `boolean`/`int`/`double`/`String`; any
  other attributed or unattributed type is `skip: default-type-unproven`
  (M0.4). The fix for an int literal given to `doubleVariation` is out of
  scope for M3.

## Decision

- **D1. Build against the frozen Central versions, not CGP.** `rewrite-java`
  and `rewrite-test` 8.90.4. No credentials are needed by contributors, CI or
  users of plugin 6.46.1 (F1). Users on the current plugin must already hold
  CGP credentials for the plugin itself; our jar still resolves from Central.
  Revisit at each release (#12); switch to CGP only if an upstream fix is
  required or F2's unverified half fails. A CGP-credentialed smoke job against
  6.71.0 is an optional follow-up, not a blocker; until it runs, "works on
  current OpenRewrite" is UNVERIFIED and must not be claimed in docs.
- **D2. Imperative `Recipe` + `JavaIsoVisitor`**, composed by one declarative
  YAML recipe. Declarative and Refaster cannot prove an OpenFeature `Client`
  is in scope or report per-call skips (analysis in issue #8).
- **D3. Hardcode the receiver FQN**
  `com.launchdarkly.sdk.server.interfaces.LDClientInterface` in the recipe, with
  `matchOverrides = true`. No schema `receiver` field: it would change both
  validators for a constant that exists only in Java. Method names come from
  `rules.yaml`. A second matcher on `(String, com.launchdarkly.sdk.LDUser, T)`
  reports `context-unproven` for v5/v6 call sites. A null method type reports
  nothing.
- **D4. Generated table, option (a):** commit `RulesGen.java`, drift-checked by
  `make spec-check`, emitted by the existing Go generator so no YAML dependency
  is added. The generator orders template arguments by placeholder appearance
  (F3). Templates are parsed with stub sources for the OpenFeature types
  (F4), never `classpath(...)`. No TypeTable.
- **D5. Two recipes, one engine.** `MigrateLaunchDarklyToOpenFeature` rewrites;
  `FindLaunchDarklyUsage` marks and writes the data table. `FindLaunchDarklyUsage`
  is documented as `dryRun`-only (`run` writes markers into source, F5) and as
  failing the build under `-DfailOnDryRunResults=true` whenever it finds
  anything, which is the intended CI gate for #12. The property is
  `failOnDryRunResults`, not `rewrite.failOnDryRunResults`.
- **D6. Artifact `io.github.jh125486:fldlint-rewrite`**, packages and recipe
  names under `io.github.jh125486.fldlint`. Explicit `@JsonCreator`, no Lombok.
  Using `io.github.jh125486` avoids the DNS-ownership question for `dev.fldlint`.
- **D7. Maven + wrapper under `java/`.** VERIFIED: JDK 21 compiles with
  `maven.compiler.release=8`, and the release-8 jar loads in the plugin on
  JDK 21. Develop on JDK 21 or newer; run tests with `maven.compiler.testRelease`
  21. Dependencies (still need written approval in CONTRIBUTING.md):
  `org.openrewrite:rewrite-java:8.90.4`; test scope `rewrite-test:8.90.4`,
  `rewrite-java-21:8.90.4`, `junit-jupiter` **5.14.4**, `slf4j-simple` (quiets
  OpenRewrite logging), and the real `launchdarkly-java-server-sdk:7.18.1` and
  `dev.openfeature:sdk:1.23.0` jars for fixtures. VERIFIED: pinning
  `junit-jupiter` 5.11.4 beside `rewrite-test`'s transitive JUnit 5.14.4 / platform
  1.14.4 crashes surefire with `NoClassDefFoundError: OutputDirectoryCreator`;
  align versions, do not import a JUnit 6 BOM.

## Consequences

- No credentials anywhere in contributor, CI or user paths, at the cost of no
  upstream fixes and the unverified compatibility with plugin 6.71.0 / core
  8.113.x. Compatibility could break silently for users on the current plugin;
  the optional CGP smoke job is the mitigation.
- Skips cannot be expressed by `rewrite:run`; they are visible only through
  `dryRun` markers, `rewrite.patch` and data tables.
- The recipe must check exact default-argument types (F6); a rewrite that
  type-checks in the template but not in the user's code is the main risk and
  needs a `RewriteTest` per boxed type.
- Boxed OpenFeature returns can be `null` under a non-LD provider and NPE when
  assigned to a primitive; document in the recipe docs.
- Rule ids `java-int-variation` and `java-json-variation` are fingerprint
  inputs; they are fixed here (#7).
- `rules.yaml` `#{name:any(T)}` placeholders (schema change in #7) are
  consumed through the generated table; they are no longer merely advisory.
