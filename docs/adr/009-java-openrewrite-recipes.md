# ADR 009: Java recipes are imperative OpenRewrite recipes built on frozen Maven Central versions

Date: 2026-11
Status: Proposed

Status stays Proposed: the owner approved the Java toolchain dependency group
(Maven, JUnit, OpenRewrite test libraries), but CONTRIBUTING.md does not list it
yet. The orchestrator records it there (issue #8); then this becomes Accepted.
Evidence (VERIFIED/UNVERIFIED): [java/spike-findings.md](../../java/spike-findings.md).

## Context

ADR 002 fixes one engine per language and the owner chose OpenRewrite for Java with
groupId `io.github.jh125486`. ADR 008 makes the Java tool follow the Maven/Gradle
rewrite plugin conventions (no new exit codes or flags).

- OpenRewrite stopped publishing to Maven Central: Central's newest `rewrite-java` is
  8.90.4 and plugin 6.46.1; current releases (core 8.113.2, plugin 6.71.0) are only on
  the credentialed Code Genome Project (CGP). Plugin 6.46.1 plus a recipe built on 8.90.4
  runs with no credentials (VERIFIED). The same jar on 6.71.0 is UNVERIFIED.
- Template placeholders `#{name:any(T)}` work, bind by order of appearance, are not
  type-checked, and need stub sources, not `classpath(...)`, to parse in the plugin.
- LaunchDarkly returns primitives (`boolean`, `int`, `double`); OpenFeature returns
  boxed `Boolean`, `Integer`, `Double`, and its boxed defaults do not widen (an `int`
  literal to `getDoubleValue` does not compile).
- OpenFeature `getXValue` returns the caller's default on any error (VERIFIED in SDK
  1.23.0 sources); a `null` value is possible only from a provider (UNVERIFIED).
- Issue #8 mentioned JDK 25; nothing here needs more than JDK 21 (D7).

## Decision

- **D1. Build on Central, not CGP:** `rewrite-java`, `rewrite-test` 8.90.4. Revisit at
  each release (#12). "Works on current OpenRewrite" must not be claimed until a
  CGP-credentialed smoke job against 6.71.0 passes.
- **D2. Imperative `Recipe` + `JavaIsoVisitor`**, composed by one declarative YAML
  recipe. Declarative and Refaster cannot prove an OpenFeature `Client` is in scope or
  report per-call skips. Declare `rewrite-java` as `provided` so the plugin's core wins.
- **D3. Hardcode the receiver** `com.launchdarkly.sdk.server.interfaces.LDClientInterface`
  with `matchOverrides = true`; no schema `receiver` field. A second matcher on
  `(String, LDUser, T)` reports `context-unproven` for v5/v6 call sites.
- **D4. Generated table:** commit `RulesGen.java`, drift-checked by `make spec-check`.
  This needs a NEW emitter in the Go generator and a `GEN_FILES` entry (the generator
  emits only `rules_gen.go` today). Args are emitted in template order; templates parse
  against minimal stub sources.
- **D5. Two recipes:** `MigrateLaunchDarklyToOpenFeature` rewrites;
  `FindLaunchDarklyUsage` marks and is `dryRun`-only (`run` writes markers into source).
  The CI gate is `-DfailOnDryRunResults=true` (not `-Drewrite.`-prefixed, silently ignored).
- **D6. Artifact `io.github.jh125486:fldlint-rewrite`**, packages `io.github.jh125486.fldlint`,
  explicit `@JsonCreator`, no Lombok.
- **D7. Maven under `java/`**, JDK 21 to build (required by `rewrite-java-21` test libs),
  `maven.compiler.release=8` for consumers. Dependencies: `rewrite-java`, test scope
  `rewrite-test`, `rewrite-java-21` (8.90.4), `junit-jupiter` 5.14.4 (aligned with
  `rewrite-test`'s JUnit), `slf4j-simple`, and the real `launchdarkly-java-server-sdk`
  7.18.1 and `dev.openfeature:sdk` 1.23.0 jars for fixtures. Pin the wrapper's
  `distributionSha256Sum`; prefer the script-only wrapper.
- **D8. Rewrites preserve primitive typing (owner decision).** Every rewritten
  `boolean`/`int`/`double` call appends `.booleanValue()`/`.intValue()`/`.doubleValue()`
  to the OpenFeature call; `String` and `Object` need none. Otherwise behavior changes
  silently: `list.remove(ld.intVariation(..))` picks `remove(Object)`, `a == b` compares
  `Integer` references, and overloads and `var` shift. A ternary `b ? ld.intVariation(..) : other`
  with an `Integer other` is an `int` conditional that throws on a null `other`; without the
  unboxing call it is an `Integer` conditional that passes null through (VERIFIED, javac 21). The rewrite only
  fires when the default is exactly `boolean`/`int`/`double`/`String`; anything else is
  `default-type-unproven`. Pinned by fixtures in `java/fixtures/primitive-typing/`:
  `remove-int`, `eq-compare`, `ternary` (plus an int-literal-to-`doubleVariation` skip,
  and `var` inference, added in #9/#10).
- **D9. Skip order is rule-level, dynamic-key, context-unproven, default-type-unproven,
  unbound-client, receiver-unproven** (ADR 010, another agent). Java applies only the
  receiver-purity half.
- **D10. Scope.** #8 (this ADR) owns the Java row of `spec/contracts/placeholders.md`
  once #7 lands the schema change; the M0.5 agent leaves it. The orchestrator adds the
  Java toolchain dependency list to CONTRIBUTING.md.

## Consequences

- No credentials for contributors, CI or plugin-6.46.1 users, but no upstream fixes and
  possible silent breakage on plugin 6.71.0 (smoke job mitigates).
- The unboxing call turns a `null` provider value into an NPE where LaunchDarkly returned a
  primitive. Mitigation: OpenFeature returns the default on error, and the rewrite never
  produces a `null` default; only a misbehaving provider can return `null` (UNVERIFIED).
  Document it in recipe docs.
- Frozen transitive deps (jackson, commons-text, antlr4, classgraph, asm, micrometer)
  cannot be bumped and will draw CVE alerts. Triage per CVE: assess reachability
  and mitigation for the specific dependency and call path (they execute inside
  the user's build plugin process, so they are not unreachable by default) and
  record the finding in release notes; never close an alert as a class.
- Gradle users are uncovered: the Plugin Portal is frozen at 7.41.0, 7.44.1 is CGP-only,
  untested. Multi-module Maven and CLI use need
  `-Drewrite.recipeArtifactCoordinates=io.github.jh125486:fldlint-rewrite:<v>` with
  `-Drewrite.activeRecipes`.
- The `dryRun` gate can pass falsely when type attribution fails (generated sources,
  Lombok, missing classpath); there is no baseline yet (#12, `spec/contracts/baseline.md`).
- Central publishing needs signing and `io.github.jh125486` namespace verification (#12).
- `rewrite:run` cannot express skips; they appear only in `dryRun` markers, patch and
  data tables. Rule ids `java-int-variation` and `java-json-variation` are fingerprint
  inputs, fixed here (#7).
