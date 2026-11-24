package io.github.jh125486.fldlint;

import static org.openrewrite.java.Assertions.java;

import com.fasterxml.jackson.databind.JsonNode;
import io.github.jh125486.fldlint.spec.SpecJson;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestFactory;
import org.openrewrite.java.JavaParser;
import org.openrewrite.test.RecipeSpec;
import org.openrewrite.test.RewriteTest;

/**
 * Conformance harness over the shared fixtures. Discovers every case in
 * spec/generated/spec.json (or the root given by {@code -Dfldlint.specRoot=<dir>}).
 * Cases with an {@code expect.java} are pending until the Java engine lands (#10);
 * the others are not applicable. The harness reads only JSON (ADR 011) and does
 * not validate against the JSON Schema: the JS loader owns validation.
 */
class ConformanceTest implements RewriteTest {

    /** The Java parser for fixtures: real SDK jars, so type attribution is complete. */
    private static JavaParser.Builder<?, ?> fixtureParser() {
        return JavaParser.fromJavaVersion().classpath("launchdarkly-java-server-sdk", "sdk");
    }

    private static boolean hasJavaExpectation(JsonNode testCase) {
        return testCase.path("case").path("expect").has("java");
    }

    @TestFactory
    List<DynamicTest> fixtures() throws IOException {
        SpecJson spec = SpecJson.load();
        List<DynamicTest> tests = new ArrayList<DynamicTest>();
        int pending = 0;
        for (JsonNode testCase : spec.cases()) {
            String path = testCase.get("path").asText();
            boolean java = hasJavaExpectation(testCase);
            if (java) {
                pending++;
            }
            tests.add(DynamicTest.dynamicTest(path, () -> {
                Assumptions.assumeTrue(java, "not applicable: no java expectation");
                Assumptions.abort("pending: Java engine lands in #10");
            }));
        }
        if (tests.isEmpty()) {
            throw new IllegalStateException("no cases in " + spec.getRoot());
        }
        System.out.printf(
                "conformance: %d cases discovered from %s: %d with a java expectation (pending #10), %d not applicable%n",
                tests.size(), spec.getRoot(), pending, tests.size() - pending);
        return tests;
    }

    /**
     * Every Java fixture file must parse with full type attribution against the
     * real SDK jars (LaunchDarkly 7.18.1, OpenFeature 1.23.0): a fixture with a
     * missing type fails here instead of silently passing. This also proves that
     * the {@code rewriteRun} default method works inside a dynamic test.
     */
    @TestFactory
    List<DynamicTest> javaFixturesTypeAttribute() throws IOException {
        SpecJson spec = SpecJson.load();
        List<DynamicTest> tests = new ArrayList<DynamicTest>();
        for (JsonNode testCase : spec.cases()) {
            if (!hasJavaExpectation(testCase)) {
                continue;
            }
            String path = testCase.get("path").asText();
            Path dir = spec.caseDir(path).resolve("java");
            for (String file : SpecJson.javaFixtureFiles(testCase)) {
                tests.add(DynamicTest.dynamicTest(path + "/" + file + " type-attributes", () -> {
                    String source = new String(Files.readAllBytes(dir.resolve(file)), StandardCharsets.UTF_8);
                    // The default recipe is a no-op: the file must parse, attribute and print back unchanged.
                    rewriteRun((RecipeSpec s) -> s.parser(fixtureParser()), java(source));
                }));
            }
        }
        return tests;
    }
}
