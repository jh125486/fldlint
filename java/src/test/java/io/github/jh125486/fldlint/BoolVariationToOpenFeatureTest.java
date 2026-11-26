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

  private String skipOf(String... bodyLines) {
    return find(body(bodyLines)).get(0).skip;
  }

  /** The expected output: the OpenFeature import added and the unused LDContext import dropped. */
  private static String migrated(String source) {
    return source
        .replace(
            "import dev.openfeature.sdk.Client;\n",
            "import dev.openfeature.sdk.Client;\nimport dev.openfeature.sdk.ImmutableContext;\n")
        .replace("import com.launchdarkly.sdk.LDContext;\n", "");
  }

  @Test
  void switchCaseGroupLocalShadowsTheField() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    boolean f(LDClient ld, String u, int n) {",
            "        switch (n) {",
            "            case 1:",
            "                String client = \"x\";",
            "                return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "            default:",
            "                return false;",
            "        }",
            "    }"));
  }

  @Test
  void switchLocalOfAnEarlierGroupIsStillInScope() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    boolean f(LDClient ld, String u, int n) {",
            "        switch (n) {",
            "            case 1:",
            "                String client = \"x\";",
            "                break;",
            "            default:",
            "                return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        }",
            "        return false;",
            "    }"));
  }

  @Test
  void switchCaseGroupClientIsTheInnermostBinding() {
    String before =
        body(
            "    boolean f(LDClient ld, String u, int n) {",
            "        switch (n) {",
            "            case 1:",
            "                Client other = OpenFeatureAPI.getInstance().getClient(\"o\");",
            "                return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "            default:",
            "                return false;",
            "        }",
            "    }");
    rewriteRun(
        BoolVariationToOpenFeatureTest::migrate,
        java(
            before,
            migrated(before)
                .replace(
                    "return ld.boolVariation(\"k\", LDContext.create(u), false);",
                    "return other.getBooleanValue(\"k\", false, new ImmutableContext(u)).booleanValue();")));
  }

  @Test
  void localIsInScopeInsideItsOwnInitializer() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    boolean f(LDClient ld, String u) {",
            "        boolean client = ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        return client;",
            "    }"));
    assertEquals(
        "unbound-client",
        skipOf(
            "    boolean f(LDClient ld, String u) {",
            "        boolean a = true, client = ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        return a && client;",
            "    }"));
  }

  @Test
  void tryResourcesAreInScopeInTheBodyButNotInCatch() {
    String[] head = {
      "    boolean f(LDClient ld, String u) {",
      "        try (java.io.StringReader client = new java.io.StringReader(\"x\")) {"
    };
    assertEquals(
        "unbound-client",
        skipOf(
            head[0],
            head[1],
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        } catch (RuntimeException e) {",
            "            return false;",
            "        }",
            "    }"));
    assertEquals(
        "",
        skipOf(
            head[0],
            head[1],
            "            return true;",
            "        } catch (RuntimeException e) {",
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        } finally {",
            "            ld.boolVariation(\"j\", LDContext.create(u), false);",
            "        }",
            "    }"));
  }

  @Test
  void fieldDeclaredAfterAFieldInitializerCallIsAForwardReference() {
    String ld = "    private final LDClient ld = null;";
    String call =
        "    private final boolean on = ld.boolVariation(\"k\", LDContext.create(\"u\"), false);";
    String late = "    private final Client late = OpenFeatureAPI.getInstance().getClient(\"l\");";
    String early =
        "    private final Client early = OpenFeatureAPI.getInstance().getClient(\"e\");";
    // The class has the field `client` from the header; a second client makes the proof ambiguous
    // unless one of them is not a usable binding at the call.
    assertEquals("", find(body(ld, call)).get(0).skip);
    assertEquals("unbound-client", find(body(ld, early, call)).get(0).skip);
    // A client that cannot be read here still makes the choice unprovable.
    assertEquals("unbound-client", find(body(ld, call, late)).get(0).skip);
  }

  @Test
  void recordComponentShadowsAnOuterClient() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    static final Client shared = OpenFeatureAPI.getInstance().getClient(\"s\");",
            "",
            "    record R(String shared) {",
            "        boolean f(LDClient ld, String u) {",
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        }",
            "    }"));
  }

  @Test
  void anotherTypeNamedImmutableContextBlocksTheRewrite() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    static class ImmutableContext {}",
            "",
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }"));
  }

  @Test
  void ldContextImportStaysWhileStillUsedElsewhere() {
    String before =
        body(
            "    static final LDContext ANON = LDContext.create(\"anon\");",
            "",
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }");
    String after =
        before
            .replace(
                "import dev.openfeature.sdk.Client;\n",
                "import dev.openfeature.sdk.Client;\nimport dev.openfeature.sdk.ImmutableContext;\n")
            .replace(
                "return ld.boolVariation(\"k\", LDContext.create(u), false);",
                "return client.getBooleanValue(\"k\", false, new ImmutableContext(u)).booleanValue();");
    rewriteRun(BoolVariationToOpenFeatureTest::migrate, java(before, after));
  }

  @Test
  void enumConstantArgumentsAreStaticAndForwardRestricted() {
    String source =
        lines(
            "package flags;",
            "",
            "import com.launchdarkly.sdk.LDContext;",
            "import com.launchdarkly.sdk.server.LDClient;",
            "import dev.openfeature.sdk.Client;",
            "import dev.openfeature.sdk.OpenFeatureAPI;",
            "",
            "public enum Mode {",
            "    A(Holder.LD.boolVariation(\"k\", LDContext.create(\"u\"), false));",
            "",
            "    static class Holder {",
            "        static final LDClient LD = null;",
            "    }",
            "",
            "    final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
            "",
            "    Mode(boolean on) {}",
            "}");
    assertEquals("unbound-client", find(source).get(0).skip);
  }

  @Test
  void aTypeParameterNamedImmutableContextBlocksTheRewrite() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    <ImmutableContext> boolean f(LDClient ld, String u, ImmutableContext x) {",
            "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }"));
  }

  @Test
  void localClientWithoutInitializerIsNotABinding() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    boolean f(LDClient ld, String u) {",
            "        Client local;",
            "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }"));
  }

  @Test
  void explicitConstructorCallArgumentsCannotUseInstanceFields() {
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
            "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
            "",
            "    Checkout(boolean on) {}",
            "",
            "    Checkout(LDClient ld, String u) {",
            "        this(ld.boolVariation(\"k\", LDContext.create(u), false));",
            "    }",
            "}");
    assertEquals("unbound-client", find(source).get(0).skip);
  }

  @Test
  void enumConstantShadowsAnOuterClient() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    static final Client shared = OpenFeatureAPI.getInstance().getClient(\"s\");",
            "",
            "    enum Mode {",
            "        shared;",
            "",
            "        boolean f(LDClient ld, String u) {",
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        }",
            "    }"));
  }

  @Test
  void contextCreateOnAnExpressionReceiverIsNotTheStaticCall() {
    assertEquals(
        "context-unproven",
        skipOf(
            "    LDContext loadContext() {",
            "        return null;",
            "    }",
            "",
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", loadContext().create(u), false);",
            "    }"));
    assertEquals(
        "context-unproven",
        skipOf(
            "    boolean f(LDClient ld, LDContext held, String u) {",
            "        return ld.boolVariation(\"k\", held.create(u), false);",
            "    }"));
    assertEquals(
        "",
        skipOf(
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", com.launchdarkly.sdk.LDContext.create(u), false);",
            "    }"));
  }

  @Test
  void inheritedFieldShadowsAnOuterClient() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    static final Client shared = OpenFeatureAPI.getInstance().getClient(\"s\");",
            "",
            "    static class Base {",
            "        protected String shared = \"x\";",
            "    }",
            "",
            "    static class Inner extends Base {",
            "        boolean f(LDClient ld, String u) {",
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        }",
            "    }"));
  }

  @Test
  void inheritedInterfaceConstantShadowsAnOuterClient() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    static final Client shared = OpenFeatureAPI.getInstance().getClient(\"s\");",
            "",
            "    interface Names {",
            "        String shared = \"x\";",
            "    }",
            "",
            "    static class Inner implements Names {",
            "        boolean f(LDClient ld, String u) {",
            "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        }",
            "    }"));
  }

  @Test
  void classWithAnUnresolvedSupertypeGivesUp() {
    final List<LaunchDarklyUsage.Row> rows = new ArrayList<LaunchDarklyUsage.Row>();
    rewriteRun(
        s ->
            s.parser(FixtureParsers.forCase(""))
                .typeValidationOptions(org.openrewrite.test.TypeValidation.none())
                .recipe(new FindLaunchDarklyUsage())
                .dataTable(LaunchDarklyUsage.Row.class, rows::addAll),
        java(
            body(
                    "    class Inner extends Missing {",
                    "        boolean f(LDClient ld, String u) {",
                    "            return ld.boolVariation(\"k\", LDContext.create(u), false);",
                    "        }",
                    "    }")
                .trim(),
            src -> src.after(actual -> actual)));
    assertEquals("unbound-client", rows.get(0).skip);
  }

  @Test
  void staticMethodOfALocalClassCannotUseAnEnclosingLocalClient() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    void outer(LDClient ld, String u) {",
            "        Client local = OpenFeatureAPI.getInstance().getClient(\"l\");",
            "        class Loc {",
            "            static boolean f(LDClient l2, String v) {",
            "                return l2.boolVariation(\"k\", LDContext.create(v), false);",
            "            }",
            "        }",
            "    }"));
  }

  @Test
  void localRecordCannotUseAnEnclosingParameterClient() {
    assertEquals(
        "unbound-client",
        skipOf(
            "    void outer(Client param) {",
            "        record Loc(String name) {",
            "            boolean f(LDClient l2, String v) {",
            "                return l2.boolVariation(\"k\", LDContext.create(v), false);",
            "            }",
            "        }",
            "    }"));
  }

  @Test
  void constructorParameterIsTheBindingInsideAnExplicitConstructorCall() {
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
            "    static final Client shared = OpenFeatureAPI.getInstance().getClient(\"s\");",
            "",
            "    Checkout(boolean on) {}",
            "",
            "    Checkout(Client param, LDClient ld, String u) {",
            "        this(ld.boolVariation(\"k\", LDContext.create(u), false));",
            "    }",
            "}");
    // The parameter is the innermost client; the static field is not chosen over it.
    assertEquals("", find(source).get(0).skip);
  }

  @Test
  void constructorParameterShadowsTheField() {
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
            "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
            "",
            "    Checkout(String client, LDClient ld, String u) {",
            "        boolean b = ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }",
            "}");
    assertEquals("unbound-client", find(source).get(0).skip);
  }

  @Test
  void blankFinalFieldIsNotABindingInConstructorsAndInitializers() {
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
            "    private final Client client;",
            "",
            "    Checkout(LDClient ld, String u) {",
            "        boolean b = ld.boolVariation(\"k\", LDContext.create(u), false);",
            "        this.client = OpenFeatureAPI.getInstance().getClient(\"c\");",
            "    }",
            "}");
    assertEquals("unbound-client", find(source).get(0).skip);
    String initializer =
        lines(
            "package flags;",
            "",
            "import com.launchdarkly.sdk.LDContext;",
            "import com.launchdarkly.sdk.server.LDClient;",
            "import dev.openfeature.sdk.Client;",
            "import dev.openfeature.sdk.OpenFeatureAPI;",
            "",
            "public class Checkout {",
            "    private final Client client;",
            "    private static final LDClient LD = null;",
            "",
            "    {",
            "        boolean b = LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
            "        client = OpenFeatureAPI.getInstance().getClient(\"c\");",
            "    }",
            "}");
    assertEquals("unbound-client", find(initializer).get(0).skip);
    String inMethod =
        lines(
            "package flags;",
            "",
            "import com.launchdarkly.sdk.LDContext;",
            "import com.launchdarkly.sdk.server.LDClient;",
            "import dev.openfeature.sdk.Client;",
            "import dev.openfeature.sdk.OpenFeatureAPI;",
            "",
            "public class Checkout {",
            "    private final Client client;",
            "",
            "    Checkout() {",
            "        client = OpenFeatureAPI.getInstance().getClient(\"c\");",
            "    }",
            "",
            "    boolean f(LDClient ld, String u) {",
            "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
            "    }",
            "}");
    assertEquals("", find(inMethod).get(0).skip);
  }

  @Test
  void instanceInitializerBlockSeesEarlierClientField() {
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
            "    private final Client early = OpenFeatureAPI.getInstance().getClient(\"e\");",
            "    private static final LDClient LD = null;",
            "",
            "    {",
            "        boolean b = LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
            "    }",
            "}");
    assertEquals("", find(source).get(0).skip);
  }

  @Test
  void blankFinalClientBlocksTheChoiceEvenAfterItIsAssigned() {
    assertEquals(
        "unbound-client",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout {",
                "    private final Client checkout;",
                "    private final Client billing = OpenFeatureAPI.getInstance().getClient(\"b\");",
                "",
                "    Checkout(LDClient ld, String u) {",
                "        checkout = OpenFeatureAPI.getInstance().getClient(\"c\");",
                "        boolean b = ld.boolVariation(\"k\", LDContext.create(u), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void explicitConstructorCallArgumentsMayUseStaticFields() {
    assertEquals(
        "",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout {",
                "    static final Client shared = OpenFeatureAPI.getInstance().getClient(\"s\");",
                "",
                "    Checkout(boolean on) {}",
                "",
                "    Checkout(LDClient ld, String u) {",
                "        this(ld.boolVariation(\"k\", LDContext.create(u), false));",
                "    }",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void anonymousClassInitializerSeesAnEffectivelyFinalEnclosingLocal() {
    assertEquals(
        "",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout {",
                "    void f(LDClient ld, String u) {",
                "        Client local = OpenFeatureAPI.getInstance().getClient(\"l\");",
                "        new Object() {",
                "            {",
                "                boolean b = ld.boolVariation(\"k\", LDContext.create(u), false);",
                "            }",
                "        };",
                "    }",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void anonymousClassConstructorArgumentsDoNotSeeTheBodyMembers() {
    assertEquals(
        "unbound-client",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout {",
                "    static class Base {",
                "        Base(boolean on) {}",
                "    }",
                "",
                "    void f(LDClient ld, String u) {",
                "        new Base(ld.boolVariation(\"k\", LDContext.create(u), false)) {",
                "            Client client = OpenFeatureAPI.getInstance().getClient(\"x\");",
                "        };",
                "    }",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void enumConstructorSeesItsInitializedInstanceField() {
    assertEquals(
        "",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public enum Checkout {",
                "    A;",
                "",
                "    private static final LDClient LD = null;",
                "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
                "",
                "    Checkout() {",
                "        boolean b = LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void compactRecordConstructorComponentClientGivesUp() {
    assertEquals(
        "unbound-client",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout {",
                "    record R(Client client) {",
                "        R {",
                "            boolean b = LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
                "        }",
                "    }",
                "",
                "    static final LDClient LD = null;",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void aSupertypeFromOutsideTheFileMayInheritAMemberTypeNamedImmutableContext() {
    assertEquals(
        "unbound-client",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout implements dev.openfeature.sdk.FeatureProvider {",
                "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
                "",
                "    boolean f(LDClient ld, String u) {",
                "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
    // JDK supertypes cannot declare one: the rewrite still applies.
    assertEquals(
        "",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public class Checkout implements Runnable {",
                "    private final Client client = OpenFeatureAPI.getInstance().getClient(\"c\");",
                "",
                "    public void run() {}",
                "",
                "    boolean f(LDClient ld, String u) {",
                "        return ld.boolVariation(\"k\", LDContext.create(u), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
  }

  @Test
  void enumConstructorsAndInstanceInitializersCannotReadStaticClients() {
    assertEquals(
        "unbound-client",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public enum Checkout {",
                "    A;",
                "",
                "    private static final LDClient LD = null;",
                "    private static final Client SHARED = OpenFeatureAPI.getInstance().getClient(\"s\");",
                "",
                "    Checkout() {",
                "        boolean b = LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
    assertEquals(
        "unbound-client",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public enum Checkout {",
                "    A;",
                "",
                "    private static final LDClient LD = null;",
                "    private static final Client SHARED = OpenFeatureAPI.getInstance().getClient(\"s\");",
                "",
                "    {",
                "        boolean b = LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
    // A static method of the enum may read its static client.
    assertEquals(
        "",
        find(lines(
                "package flags;",
                "",
                "import com.launchdarkly.sdk.LDContext;",
                "import com.launchdarkly.sdk.server.LDClient;",
                "import dev.openfeature.sdk.Client;",
                "import dev.openfeature.sdk.OpenFeatureAPI;",
                "",
                "public enum Checkout {",
                "    A;",
                "",
                "    private static final LDClient LD = null;",
                "    private static final Client SHARED = OpenFeatureAPI.getInstance().getClient(\"s\");",
                "",
                "    static boolean f() {",
                "        return LD.boolVariation(\"k\", LDContext.create(\"u\"), false);",
                "    }",
                "}"))
            .get(0)
            .skip);
  }
}
