package io.github.jh125486.fldlint;

import org.openrewrite.java.JavaParser;

/**
 * Minimal OpenFeature sources for template parsing. {@code classpath("sdk")} works in tests but not
 * in the Maven plugin (spike F4), while {@code dependsOn} works in both. Kept in step with
 * dev.openfeature:sdk 1.23.0 by the conformance fixtures, which compile against the real jar.
 */
final class OpenFeatureStubs {
  private static final String[] SOURCES = {
    "package dev.openfeature.sdk; public interface EvaluationContext {}",
    "package dev.openfeature.sdk;"
        + " public class ImmutableContext implements EvaluationContext {"
        + " public ImmutableContext(String targetingKey) {} }",
    "package dev.openfeature.sdk;"
        + " public interface Client {"
        + " Boolean getBooleanValue(String key, Boolean defaultValue, EvaluationContext ctx); }"
  };

  private OpenFeatureStubs() {}

  static JavaParser.Builder<?, ?> parser() {
    return JavaParser.fromJavaVersion().dependsOn(SOURCES);
  }
}
