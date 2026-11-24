# ADR 012: Java formatting with Spotless and static analysis with Error Prone

Date: 2026-11
Status: Accepted

Addendum to [ADR 009](009-java-openrewrite-recipes.md), which names no formatter or
analyzer (issue #44). ADR 009 is unchanged.

## Context

The `java/` module compiled with `-Xlint:all -Werror` only. We want one enforced
formatter and one static analyzer, both runnable offline from Maven, on JDK 21 builds
with `maven.compiler.release=8` main code.

## Decision

- **Formatter: Spotless** (`spotless-maven-plugin` 3.12.0) with google-java-format 2.0.0
  and unused-import removal. `spotless:check` is bound to `validate`, so every
  `./mvnw` build and the `lint` profile fail fast on unformatted code. Fix with
  `cd java && ./mvnw spotless:apply`. The generated `RulesGen.java` is excluded: the Go
  generator owns it and `task spec:check` drift-checks it.
- **Analyzer: Error Prone** 2.53.0 (`error_prone_core`), not SpotBugs. It runs inside
  javac, so it needs no bytecode pass and no extra plugin, reports on the source with
  precise positions, and composes with the existing `-Xlint:all -Werror` `lint` profile
  (own `target-lint` directory, so lint and test still run in parallel). It runs only in
  the `lint` profile, keeping the default build and tests free of it. SpotBugs would add
  a second plugin and a bytecode scan that ignores the release-8 source level.
- **JDK requirements:** Error Prone 2.53.0 runs on JDK 17+; we build on JDK 21 and it
  analyzes release-8 sources fine. On JDK 21 it needs the forked compiler flags
  `-XDcompilePolicy=simple`, `--should-stop=ifError=FLOW`, `-XDaddTypeAnnotationsToSymbol=true`,
  and `-J--add-exports`/`-J--add-opens` for the `jdk.compiler` packages it uses (listed in
  `java/pom.xml`). Spotless needs no such flags.
- Findings are fixed in code. Suppress only with a narrow `@SuppressWarnings` plus a
  reason; `RulesGen.java` is excluded via `-XepExcludedPaths`.
- Versions are Maven Central's newest stable releases at 2026-11-24; bumping them
  needs no ADR, but adding any other build-time dependency needs owner approval.

## Consequences

- New build-time dependencies, listed in `CONTRIBUTING.md` and `docs/m0-contract.md` (M0.8).
- Unformatted Java fails `task java:lint`, `task java:test` and CI.
- The Error Prone flag set is JDK-specific and must be revisited when the build JDK
  changes.
