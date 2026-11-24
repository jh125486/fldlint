package io.github.jh125486.fldlint;

import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.openrewrite.java.Assertions.java;

import java.util.Arrays;
import java.util.List;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;
import org.openrewrite.java.ChangeType;
import org.openrewrite.test.RecipeSpec;
import org.openrewrite.test.RewriteTest;

/**
 * Proves the assumption behind the conformance harness (issue #9): the {@code rewriteRun} default
 * method of {@link RewriteTest} works when called from inside a JUnit 5 {@link DynamicTest}
 * executable, both when the expected output matches and when it does not (the assertion must still
 * fire).
 */
class RewriteInDynamicTestTest implements RewriteTest {

  private static final String BEFORE = "package a;\nclass A {\n    java.util.Date d;\n}\n";
  private static final String AFTER =
      "package a;\n\nimport java.sql.Date;\n\nclass A {\n    Date d;\n}\n";

  private static void changeDate(RecipeSpec spec) {
    spec.recipe(new ChangeType("java.util.Date", "java.sql.Date", false));
  }

  @TestFactory
  List<DynamicTest> rewriteRunInsideDynamicTest() {
    return Arrays.asList(
        DynamicTest.dynamicTest(
            "rewrite matches the expected output",
            () -> rewriteRun(RewriteInDynamicTestTest::changeDate, java(BEFORE, AFTER))),
        DynamicTest.dynamicTest(
            "unchanged source with a no-op recipe", () -> rewriteRun(java(AFTER))),
        DynamicTest.dynamicTest(
            "a wrong expected output still fails",
            () ->
                assertThrows(
                    AssertionError.class,
                    () ->
                        rewriteRun(
                            RewriteInDynamicTestTest::changeDate,
                            java(BEFORE, BEFORE + "// wrong\n")))));
  }
}
