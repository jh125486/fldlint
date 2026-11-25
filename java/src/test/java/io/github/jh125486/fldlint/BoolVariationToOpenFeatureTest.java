package io.github.jh125486.fldlint;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.openrewrite.java.Assertions.java;

import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.openrewrite.test.RecipeSpec;
import org.openrewrite.test.RewriteTest;

/**
 * Behaviour beyond the shared fixtures: scope walking edge cases, comment placement, the data
 * table, and calls that are not SDK calls at all.
 */
class BoolVariationToOpenFeatureTest implements RewriteTest {
  private static final String HEADER =
      lines(
          "package flags;",
          "",
          "import com.launchdarkly.sdk.LDContext;",
          "import com.launchdarkly.sdk.server.LDClient;",
          "import com.launchdarkly.sdk.server.interfaces.LDClientInterface;",
          "import dev.openfeature.sdk.Client;",
          "import dev.openfeature.sdk.OpenFeatureAPI;",
          "import java.util.function.Supplier;",
          "",
          "public class Checkout {",
          "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
          "");

  private static String lines(String... lines) {
    StringBuilder sb = new StringBuilder();
    for (String line : lines) {
      sb.append(line).append('\n');
    }
    return sb.toString();
  }

  private static String body(String... lines) {
    return HEADER + lines(lines) + "}\n";
  }

  private static void migrate(RecipeSpec spec) {
    spec.parser(FixtureParsers.forCase("")).recipe(new BoolVariationToOpenFeature());
  }

  private List<LaunchDarklyUsage.Row> find(String source) {
    final List<LaunchDarklyUsage.Row> rows = new ArrayList<LaunchDarklyUsage.Row>();
    rewriteRun(
        s ->
            s.parser(FixtureParsers.forCase(""))
                .recipe(new FindLaunchDarklyUsage())
                .dataTable(LaunchDarklyUsage.Row.class, rows::addAll),
        java(source.trim(), src -> src.after(actual -> actual)));
    return rows;
  }

  @Test
  void commentInTheReceiverGapIsReportedWithNoFixAndLeftInPlace() {
    String source =
        body(
            "    boolean f(LDClient ld, String u) {",
            "        return ld /* a */ .boolVariation(\"k\", LDContext.create(u), false);",
            "    }");
    rewriteRun(BoolVariationToOpenFeatureTest::migrate, java(source));
    List<LaunchDarklyUsage.Row> rows = find(source);
    assertEquals(1, rows.size());
    assertEquals("", rows.get(0).skip);
    assertEquals("comment", rows.get(0).noFix);
    assertEquals(
        "Migrate boolean flag \"k\" to OpenFeature [no-fix: comment]", rows.get(0).message);
  }

  @Test
  void commentBesideAnArgumentIsReportedWithNoFix() {
    String source =
        body(
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", /* who */ LDContext.create(u), false);",
            "    }");
    rewriteRun(BoolVariationToOpenFeatureTest::migrate, java(source));
    assertEquals("comment", find(source).get(0).noFix);
  }

  @Test
  void commentInsideLdContextCreateOutsideItsKeyIsReportedWithNoFix() {
    String source =
        body(
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", LDContext.create(/* k */ u), false);",
            "    }");
    rewriteRun(BoolVariationToOpenFeatureTest::migrate, java(source));
    assertEquals("comment", find(source).get(0).noFix);
  }

  @Test
  void commentInsideAnArgumentsOwnTextIsCopiedWithIt() {
    rewriteRun(
        BoolVariationToOpenFeatureTest::migrate,
        java(
            body(
                "    boolean f(LDClient ld, String u, boolean a, boolean b) {",
                "        return ld.boolVariation(\"k\", LDContext.create(u), a /* x */ && b);",
                "    }"),
            body(
                    "    boolean f(LDClient ld, String u, boolean a, boolean b) {",
                    "        return client.getBooleanValue(\"k\", a /* x */ && b, new ImmutableContext(u)).booleanValue();",
                    "    }")
                .replace(
                    "import dev.openfeature.sdk.Client;\n",
                    "import dev.openfeature.sdk.Client;\nimport dev.openfeature.sdk.ImmutableContext;\n")
                .replace("import com.launchdarkly.sdk.LDContext;\n", "")));
  }

  @Test
  void localClientCapturedByALambdaIsBoundWhenEffectivelyFinal() {
    String source =
        body(
            "    Supplier<Boolean> f(LDClient ld, String u) {",
            "        Client local = OpenFeatureAPI.getInstance().getClient(\"x\");",
            "        return () -> ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }");
    List<LaunchDarklyUsage.Row> rows = find(source);
    // Two clients are visible (the field and the local); the innermost scope that has one wins.
    assertEquals(1, rows.size());
    assertEquals("", rows.get(0).skip);
  }

  @Test
  void localClientBeatsTheFieldAndTheCallRewritesToIt() {
    rewriteRun(
        BoolVariationToOpenFeatureTest::migrate,
        java(
            body(
                "    boolean f(LDClient ld, String u) {",
                "        Client local = OpenFeatureAPI.getInstance().getClient(\"x\");",
                "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
                "    }"),
            body(
                    "    boolean f(LDClient ld, String u) {",
                    "        Client local = OpenFeatureAPI.getInstance().getClient(\"x\");",
                    "        return local.getBooleanValue(\"k\", false, new ImmutableContext(u)).booleanValue();",
                    "    }")
                .replace(
                    "import dev.openfeature.sdk.Client;\n",
                    "import dev.openfeature.sdk.Client;\nimport dev.openfeature.sdk.ImmutableContext;\n")
                .replace("import com.launchdarkly.sdk.LDContext;\n", "")));
  }

  @Test
  void aClientDeclaredAfterTheCallIsNotInScope() {
    String source =
        lines(
            "package flags;",
            "",
            "import com.launchdarkly.sdk.LDContext;",
            "import com.launchdarkly.sdk.server.LDClient;",
            "import dev.openfeature.sdk.Client;",
            "import dev.openfeature.sdk.OpenFeatureAPI;",
            "",
            "public class Checkout {",
            "    boolean f(LDClient ld, String u) {",
            "        boolean on = ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        Client late = OpenFeatureAPI.getInstance().getClient(\"x\");",
            "        return on;",
            "    }",
            "}",
            "");
    assertEquals("unbound-client", find(source).get(0).skip);
  }

  @Test
  void typeBoundByAnInstanceofPatternWithTheClientsNameShadowsIt() {
    String source =
        body(
            "    boolean f(LDClient ld, String u, Object o) {",
            "        if (o instanceof String client) {",
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        }",
            "        return false;",
            "    }");
    assertEquals("unbound-client", find(source).get(0).skip);
  }

  @Test
  void receiverTypedAsTheInterfaceIsMatchedThroughOverrides() {
    List<LaunchDarklyUsage.Row> rows =
        find(
            body(
                "    boolean f(LDClientInterface ld, String u) {",
                "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
                "    }"));
    assertEquals(1, rows.size());
    assertEquals("", rows.get(0).skip);
  }

  @Test
  void aMethodNamedLikeTheSdkOnAnotherTypeIsNotACall() {
    String source =
        lines(
            "package flags;",
            "",
            "public class Other {",
            "    boolean boolVariation(String k, Object c, boolean d) {",
            "        return d;",
            "    }",
            "",
            "    boolean f() {",
            "        return boolVariation(\"k\", null, false);",
            "    }",
            "}",
            "");
    // No finding means the search recipe changes nothing.
    rewriteRun(
        s -> s.parser(FixtureParsers.forCase("")).recipe(new FindLaunchDarklyUsage()),
        java(source));
    rewriteRun(BoolVariationToOpenFeatureTest::migrate, java(source));
  }

  @Test
  void rowsCarryPositionMessageAndTheFingerprintOfTheirOccurrence() {
    String source =
        body(
            "    boolean f(LDClient ld, String u, String dyn) {",
            "        boolean a = ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        boolean b = ld.boolVariation(\"k\", LDContext.create(u), true);",
            "        boolean c = ld.boolVariation(dyn, LDContext.create(u), true);",
            "        return a && b && c;",
            "    }");
    List<LaunchDarklyUsage.Row> rows = find(source);
    assertEquals(3, rows.size());
    String path = rows.get(0).sourcePath;
    assertEquals("Migrate boolean flag \"k\" to OpenFeature", rows.get(0).message);
    assertEquals(Fingerprint.of("bool-variation", path, "k", 0), rows.get(0).fingerprint);
    assertEquals(Fingerprint.of("bool-variation", path, "k", 1), rows.get(1).fingerprint);
    assertEquals("dynamic-key", rows.get(2).skip);
    assertEquals("Flag key is not a static string [skip: dynamic-key]", rows.get(2).message);
    assertEquals(Fingerprint.of("bool-variation", path, null, 0), rows.get(2).fingerprint);
  }
}
