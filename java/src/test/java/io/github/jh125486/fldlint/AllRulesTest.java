package io.github.jh125486.fldlint;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.openrewrite.java.Assertions.java;

import io.github.jh125486.fldlint.planner.CallSite;
import io.github.jh125486.fldlint.planner.PlanResult;
import io.github.jh125486.fldlint.planner.Planner;
import io.github.jh125486.fldlint.spec.Binding;
import io.github.jh125486.fldlint.spec.Rule;
import io.github.jh125486.fldlint.spec.RulesGen;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.openrewrite.Recipe;
import org.openrewrite.test.RecipeSpec;
import org.openrewrite.test.RewriteTest;

/**
 * The table-driven Java rules (#11): every rule with a Java binding in the generated table is
 * planned, detected and either rewritten or only reported. The cases are built from {@link
 * RulesGen}, so a rule added to the spec without a Java test case here fails.
 */
class AllRulesTest implements RewriteTest {
  private static final String HEADER =
      "package flags;\n"
          + "import com.launchdarkly.sdk.LDContext;\n"
          + "import com.launchdarkly.sdk.LDValue;\n"
          + "import com.launchdarkly.sdk.server.FlagsStateOption;\n"
          + "import com.launchdarkly.sdk.server.LDClient;\n"
          + "import com.launchdarkly.sdk.server.MigrationStage;\n"
          + "import dev.openfeature.sdk.Client;\n"
          + "import dev.openfeature.sdk.OpenFeatureAPI;\n"
          + "public class Checkout {\n"
          + "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");\n";

  private static final String[][] REPORT_ONLY = {
    // rule id, call expression on ld (an LDClient) with ctx an LDContext
    {"bool-variation-detail", "ld.boolVariationDetail(\"k\", ctx, false)"},
    {"string-variation-detail", "ld.stringVariationDetail(\"k\", ctx, \"d\")"},
    {"number-variation-detail", "ld.doubleVariationDetail(\"k\", ctx, 1.5)"},
    {"int-variation-detail", "ld.intVariationDetail(\"k\", ctx, 3)"},
    {"json-variation-detail", "ld.jsonValueVariationDetail(\"k\", ctx, LDValue.ofNull())"},
    {"java-json-variation", "ld.jsonValueVariation(\"k\", ctx, LDValue.ofNull())"},
    {"migration-variation", "ld.migrationVariation(\"k\", ctx, MigrationStage.OFF)"},
    {"all-flags-state", "ld.allFlagsState(ctx)"},
    {"all-flags-state", "ld.allFlagsState(ctx, FlagsStateOption.WITH_REASONS)"},
  };

  private static final String[][] REWRITING = {
    // rule id, recipe class, before call, after call
    {
      "bool-variation",
      "BoolVariationToOpenFeature",
      "ld.boolVariation(\"k\", LDContext.create(u), false)",
      "client.getBooleanValue(\"k\", false, new ImmutableContext(u)).booleanValue()"
    },
    {
      "string-variation",
      "StringVariationToOpenFeature",
      "ld.stringVariation(\"k\", LDContext.create(u), \"d\")",
      "client.getStringValue(\"k\", \"d\", new ImmutableContext(u))"
    },
    {
      "number-variation",
      "NumberVariationToOpenFeature",
      "ld.doubleVariation(\"k\", LDContext.create(u), 1.5)",
      "client.getDoubleValue(\"k\", 1.5, new ImmutableContext(u)).doubleValue()"
    },
    {
      "java-int-variation",
      "IntVariationToOpenFeature",
      "ld.intVariation(\"k\", LDContext.create(u), 3)",
      "client.getIntegerValue(\"k\", 3, new ImmutableContext(u)).intValue()"
    },
  };

  private static String source(String call) {
    return HEADER
        + "    Object f(LDClient ld, LDContext ctx, String u) {\n"
        + "        return "
        + call
        + ";\n"
        + "    }\n"
        + "}\n";
  }

  private static Recipe recipe(String simpleName) throws ReflectiveOperationException {
    return Class.forName("io.github.jh125486.fldlint." + simpleName)
        .asSubclass(Recipe.class)
        .getDeclaredConstructor()
        .newInstance();
  }

  private List<LaunchDarklyUsage.Row> find(String source) {
    final List<LaunchDarklyUsage.Row> rows = new ArrayList<LaunchDarklyUsage.Row>();
    rewriteRun(
        s ->
            s.parser(FixtureParsers.forCase(""))
                .recipe(new FindLaunchDarklyUsage())
                .dataTable(LaunchDarklyUsage.Row.class, rows::addAll),
        java(source, src -> src.after(actual -> actual)));
    return rows;
  }

  @Test
  void everyJavaRuleHasATestCase() {
    List<String> covered = new ArrayList<String>();
    for (String[] row : REPORT_ONLY) {
      covered.add(row[0]);
    }
    for (String[] row : REWRITING) {
      covered.add(row[0]);
    }
    for (Rule rule : RulesGen.RULES) {
      if (rule.getLanguages().containsKey("java")) {
        assertTrue(covered.contains(rule.getId()), "no Java test case for rule " + rule.getId());
      }
    }
  }

  @Test
  void everyJavaReportOnlyBindingHasArgCountAndPlansItsSkip() {
    for (Rule rule : RulesGen.RULES) {
      Binding binding = rule.getLanguages().get("java");
      if (binding == null || rule.getSkip().isEmpty()) {
        continue;
      }
      assertTrue(
          binding.getArgCountMin() != null, "argCount missing on java binding " + rule.getId());
      PlanResult plan = Planner.plan(CallSite.builder(rule.getId()).build());
      assertEquals(PlanResult.Kind.SKIP, plan.getKind(), rule.getId());
      assertEquals(rule.getSkip(), plan.getCategory(), rule.getId());
    }
  }

  @Test
  void reportOnlyCallsAreReportedWithTheRuleSkipAndNeverRewritten() throws Exception {
    for (String[] row : REPORT_ONLY) {
      Rule rule = Planner.lookupRule(row[0]);
      String src = source(row[1]);
      List<LaunchDarklyUsage.Row> rows = find(src);
      assertEquals(1, rows.size(), row[1]);
      assertEquals(row[0], rows.get(0).ruleId, row[1]);
      assertEquals(rule.getSkip(), rows.get(0).skip, row[1]);
      // None of the rewriting recipes changes the file.
      for (String[] rewriting : REWRITING) {
        final Recipe recipe = recipe(rewriting[1]);
        rewriteRun(
            (RecipeSpec s) -> s.parser(FixtureParsers.forCase("")).recipe(recipe), java(src));
      }
    }
  }

  @Test
  void rewritingRulesRewriteAndAreIdempotent() throws Exception {
    for (String[] row : REWRITING) {
      final Recipe recipe = recipe(row[1]);
      String before = source(row[2]);
      rewriteRun(
          (RecipeSpec s) -> s.parser(FixtureParsers.forCase("")).recipe(recipe),
          java(
              before,
              src ->
                  src.after(actual -> {
                    assertTrue(actual.contains(row[3]), actual);
                    assertTrue(actual.contains("import dev.openfeature.sdk.ImmutableContext;"));
                    return actual;
                  })));
      // The call is also reported as rewritable by the search recipe.
      List<LaunchDarklyUsage.Row> rows = find(before);
      assertEquals(1, rows.size(), row[2]);
      assertEquals(row[0], rows.get(0).ruleId);
      assertEquals("", rows.get(0).skip);
    }
  }

  @Test
  void defaultOfAnotherTypeIsNotProven() throws Exception {
    String[][] cases = {
      // rule, call with a default whose static type is not the rule's type
      {"number-variation", "ld.doubleVariation(\"k\", LDContext.create(u), 1)"},
      {"number-variation", "ld.doubleVariation(\"k\", LDContext.create(u), 1.5f)"},
      {"java-int-variation", "ld.intVariation(\"k\", LDContext.create(u), 'a')"},
      {"java-int-variation", "ld.intVariation(\"k\", LDContext.create(u), (short) 1)"},
      {"java-int-variation", "ld.intVariation(\"k\", LDContext.create(u), Integer.valueOf(1))"},
      {"number-variation", "ld.doubleVariation(\"k\", LDContext.create(u), Double.valueOf(1))"},
    };
    for (String[] row : cases) {
      List<LaunchDarklyUsage.Row> rows = find(source(row[1]));
      assertEquals(1, rows.size(), row[1]);
      assertEquals(row[0], rows.get(0).ruleId, row[1]);
      assertEquals("default-type-unproven", rows.get(0).skip, row[1]);
    }
  }

  @Test
  void methodOfTheSameNameOnAnotherClassGivesNoFinding() {
    StringBuilder methods = new StringBuilder();
    StringBuilder calls = new StringBuilder();
    for (Rule rule : RulesGen.RULES) {
      Binding binding = rule.getLanguages().get("java");
      if (binding == null) {
        continue;
      }
      for (String name : binding.getMethods()) {
        methods
            .append("        Object ")
            .append(name)
            .append("(String k, int c, int d) { return k; }\n");
        calls.append("        t.").append(name).append("(\"k\", 1, 2);\n");
      }
    }
    String src =
        "package flags;\n"
            + "public class Checkout {\n"
            + "    static class Toggles {\n"
            + methods
            + "    }\n"
            + "    void f(Toggles t) {\n"
            + calls
            + "    }\n"
            + "}\n";
    // Nothing is found, so nothing is marked and the source stays as it is.
    rewriteRun(
        s -> s.parser(FixtureParsers.forCase("")).recipe(new FindLaunchDarklyUsage()), java(src));
  }

  private static String threeCalls(String first, String second, String third) {
    return HEADER
        + "    void f(LDClient ld, LDContext ctx) {\n"
        + "        "
        + first
        + ";\n        "
        + second
        + ";\n        "
        + third
        + ";\n    }\n}\n";
  }

  @Test
  void reportOnlyFindingsKeepTheirLiteralKeyInTheFingerprint() {
    // a, b, a: different keys differ, and the same key is told apart by its occurrence.
    String[][] rules = {
      {"bool-variation-detail", "ld.boolVariationDetail(\"%s\", ctx, false)"},
      {"java-json-variation", "ld.jsonValueVariation(\"%s\", ctx, LDValue.ofNull())"},
      {"migration-variation", "ld.migrationVariation(\"%s\", ctx, MigrationStage.OFF)"},
    };
    for (String[] rule : rules) {
      List<LaunchDarklyUsage.Row> rows =
          find(
              threeCalls(
                  String.format(rule[1], "a"),
                  String.format(rule[1], "b"),
                  String.format(rule[1], "a")));
      assertEquals(3, rows.size(), rule[0]);
      String path = rows.get(0).sourcePath;
      assertEquals(Fingerprint.of(rule[0], path, "a", 0), rows.get(0).fingerprint, rule[0]);
      assertEquals(Fingerprint.of(rule[0], path, "b", 0), rows.get(1).fingerprint, rule[0]);
      assertEquals(Fingerprint.of(rule[0], path, "a", 1), rows.get(2).fingerprint, rule[0]);
    }
  }

  @Test
  void dynamicKeyAndBulkReportsStayKeyless() {
    String src =
        HEADER
            + "    void f(LDClient ld, LDContext ctx, String k) {\n"
            + "        ld.boolVariationDetail(k, ctx, false);\n"
            + "        ld.allFlagsState(ctx);\n"
            + "    }\n}\n";
    List<LaunchDarklyUsage.Row> rows = find(src);
    assertEquals(2, rows.size());
    String path = rows.get(0).sourcePath;
    assertEquals(Fingerprint.of("bool-variation-detail", path, null, 0), rows.get(0).fingerprint);
    assertEquals(Fingerprint.of("all-flags-state", path, null, 0), rows.get(1).fingerprint);
  }
}
