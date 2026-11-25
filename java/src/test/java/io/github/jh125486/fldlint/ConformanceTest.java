package io.github.jh125486.fldlint;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.openrewrite.java.Assertions.java;

import com.fasterxml.jackson.databind.JsonNode;
import io.github.jh125486.fldlint.spec.SpecJson;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.List;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;
import org.openrewrite.java.search.FindMissingTypes;
import org.openrewrite.test.RecipeSpec;
import org.openrewrite.test.RewriteTest;

/**
 * Conformance harness over the shared fixtures. Discovers every case in spec/generated/spec.json
 * (or the root given by {@code -Dfldlint.specRoot=<dir>}). A case with an {@code expect.java}
 * checks, with real OpenRewrite runs: the diagnostics (line, col, ruleId, skip) reported by {@link
 * FindLaunchDarklyUsage}, then the output of the migration recipe against {@code After.java} (or no
 * change at all for a skip), then that the recipe does not change {@code After.java} again.
 *
 * <p>Run with {@code -Dfldlint.updateFixtures=true} to write the recipe's real output to {@code
 * After.java}; review the diff before committing. The harness reads only JSON (ADR 011) and does
 * not validate against the JSON Schema: the JS loader owns validation.
 */
class ConformanceTest implements RewriteTest {
  private static final String MIGRATE =
      "io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature";
  private static final String UPDATE_PROPERTY = "fldlint.updateFixtures";

  private static boolean hasJavaExpectation(JsonNode testCase) {
    return testCase.path("case").path("expect").has("java");
  }

  private static String read(Path file) throws IOException {
    return new String(Files.readAllBytes(file), StandardCharsets.UTF_8);
  }

  @TestFactory
  List<DynamicTest> fixtures() throws IOException {
    final SpecJson spec = SpecJson.load();
    List<DynamicTest> tests = new ArrayList<DynamicTest>();
    int applicable = 0;
    for (final JsonNode testCase : spec.cases()) {
      final String path = testCase.get("path").asText();
      final boolean java = hasJavaExpectation(testCase);
      if (java) {
        applicable++;
      }
      tests.add(
          DynamicTest.dynamicTest(
              path,
              () -> {
                Assumptions.assumeTrue(java, "not applicable: no java expectation");
                check(spec, testCase, path);
              }));
    }
    if (tests.isEmpty()) {
      throw new IllegalStateException("no cases in " + spec.getRoot());
    }
    System.out.printf(
        "conformance: %d cases discovered from %s: %d with a java expectation, %d not applicable%n",
        tests.size(), spec.getRoot(), applicable, tests.size() - applicable);
    return tests;
  }

  private void check(SpecJson spec, JsonNode testCase, String path) throws IOException {
    JsonNode expect = testCase.path("case").path("expect").path("java");
    Path dir = spec.caseDir(path).resolve("java");
    String before = read(dir.resolve("Before.java"));
    boolean output = expect.path("output").asBoolean(false);
    Path afterFile = dir.resolve("After.java");
    boolean update = Boolean.getBoolean(UPDATE_PROPERTY);

    checkDiagnostics(path, expect.path("diagnostics"), before);

    if (!output) {
      // A skip: the recipe must not change the file.
      rewriteRun(s -> migrate(s, path), java(before));
      return;
    }
    if (update) {
      rewriteRun(
          s -> migrate(s, path),
          java(
              before,
              src ->
                  src.after(actual -> {
                    try {
                      Files.write(afterFile, actual.getBytes(StandardCharsets.UTF_8));
                    } catch (IOException e) {
                      throw new java.io.UncheckedIOException(e);
                    }
                    return actual;
                  })));
    }
    String after = read(afterFile);
    rewriteRun(s -> migrate(s, path), java(before, after));
    // Idempotency: the migrated file is left alone (RewriteTest also fails a second cycle that
    // changes the output; this states it per fixture).
    rewriteRun(s -> migrate(s, path), java(after));
  }

  private static void migrate(RecipeSpec spec, String path) {
    spec.parser(FixtureParsers.forCase(path))
        .recipeFromResource("/META-INF/rewrite/fldlint.yml", MIGRATE);
  }

  private void checkDiagnostics(String path, JsonNode expected, String before) {
    final List<LaunchDarklyUsage.Row> rows = new ArrayList<LaunchDarklyUsage.Row>();
    rewriteRun(
        s ->
            s.parser(FixtureParsers.forCase(path))
                .recipe(new FindLaunchDarklyUsage())
                .dataTable(LaunchDarklyUsage.Row.class, rows::addAll),
        // The search recipe marks the source; the marked text is not under test here.
        java(before, src -> src.after(actual -> actual)));
    Collections.sort(
        rows,
        new Comparator<LaunchDarklyUsage.Row>() {
          @Override
          public int compare(LaunchDarklyUsage.Row a, LaunchDarklyUsage.Row b) {
            return a.line != b.line
                ? Integer.compare(a.line, b.line)
                : Integer.compare(a.col, b.col);
          }
        });
    assertEquals(expected.size(), rows.size(), path + ": number of diagnostics");
    for (int i = 0; i < rows.size(); i++) {
      LaunchDarklyUsage.Row row = rows.get(i);
      JsonNode want = expected.get(i);
      assertEquals(want.get("line").asInt(), row.line, path + ": line of diagnostic " + i);
      assertEquals(want.get("col").asInt(), row.col, path + ": col of diagnostic " + i);
      assertEquals(want.get("ruleId").asText(), row.ruleId, path + ": ruleId of diagnostic " + i);
      assertEquals(want.path("skip").asText(""), row.skip, path + ": skip of diagnostic " + i);
      assertEquals("", row.noFix, path + ": no fixture expects no-fix");
    }
  }

  /**
   * Every Java fixture file must parse with full type attribution against the real SDK jars
   * (LaunchDarkly 7.18.1, OpenFeature 1.23.0; the v6 stubs for the LDUser case): {@code
   * FindMissingTypes} reports nothing. A fixture with a missing type fails here instead of silently
   * passing. This also proves that the {@code rewriteRun} default method works inside a dynamic
   * test.
   */
  @TestFactory
  List<DynamicTest> javaFixturesTypeAttribute() throws IOException {
    SpecJson spec = SpecJson.load();
    List<DynamicTest> tests = new ArrayList<DynamicTest>();
    for (JsonNode testCase : spec.cases()) {
      if (!hasJavaExpectation(testCase)) {
        continue;
      }
      final String path = testCase.get("path").asText();
      final Path dir = spec.caseDir(path).resolve("java");
      for (final String file : SpecJson.javaFixtureFiles(testCase)) {
        tests.add(
            DynamicTest.dynamicTest(
                path + "/" + file + " type-attributes",
                () ->
                    rewriteRun(
                        (RecipeSpec s) ->
                            s.parser(FixtureParsers.forCase(path))
                                .recipe(new FindMissingTypes(false)),
                        java(read(dir.resolve(file))))));
      }
    }
    return tests;
  }
}
