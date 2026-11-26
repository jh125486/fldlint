package io.github.jh125486.fldlint;

import io.github.jh125486.fldlint.planner.PlanResult;
import io.github.jh125486.fldlint.planner.Planner;
import io.github.jh125486.fldlint.spec.Binding;
import io.github.jh125486.fldlint.spec.Rule;
import org.openrewrite.ExecutionContext;
import org.openrewrite.Recipe;
import org.openrewrite.TreeVisitor;
import org.openrewrite.java.JavaIsoVisitor;
import org.openrewrite.java.JavaTemplate;
import org.openrewrite.java.tree.Expression;
import org.openrewrite.java.tree.J;
import org.openrewrite.java.tree.Space;

/**
 * Rewrites {@code ldClient.boolVariation("key", LDContext.create(k), dflt)} to {@code
 * client.getBooleanValue("key", dflt, new ImmutableContext(k)).booleanValue()}, only for calls the
 * {@link Planner} says to rewrite. Argument source is copied as the original tree elements, never
 * re-printed (placeholders.md).
 */
public class BoolVariationToOpenFeature extends Recipe {
  private static final String RULE_ID = "bool-variation";
  private static final String IMPORT_CONTEXT = "dev.openfeature.sdk.ImmutableContext";

  /** The converted context (its own template; ADR 006), substituted for {@code #{context}}. */
  private static final String CONTEXT_TEMPLATE = "new ImmutableContext(#{any(java.lang.String)})";

  @Override
  public String getDisplayName() {
    return "Migrate LaunchDarkly boolVariation to OpenFeature";
  }

  @Override
  public String getDescription() {
    return "Rewrites LDClientInterface.boolVariation(key, LDContext.create(k), default) to an"
        + " OpenFeature Client.getBooleanValue call when the key is a string literal, the context"
        + " is LDContext.create(k), the default is a boolean and exactly one OpenFeature client is"
        + " in scope. Other calls are left alone; run FindLaunchDarklyUsage to see why.";
  }

  /** The rule's Java template with each placeholder given a type, by order of appearance. */
  static String expandTemplate(Rule rule) {
    Binding binding = rule.getLanguages().get("java");
    return binding
        .getTemplate()
        .replace("#{key}", "#{any(java.lang.String)}")
        .replace("#{default}", "#{any(" + defaultType(rule.getValueType()) + ")}")
        .replace("#{context}", CONTEXT_TEMPLATE);
  }

  private static String defaultType(String valueType) {
    switch (valueType) {
      case "boolean":
        return "boolean";
      case "integer":
        return "int";
      case "number":
        return "double";
      default:
        return "java.lang.String";
    }
  }

  @Override
  public TreeVisitor<?, ExecutionContext> getVisitor() {
    Rule rule = Planner.lookupRule(RULE_ID);
    final JavaTemplate template =
        JavaTemplate.builder(expandTemplate(rule))
            .imports(IMPORT_CONTEXT)
            .javaParser(OpenFeatureStubs.parser())
            .build();
    return new JavaIsoVisitor<ExecutionContext>() {
      @Override
      public J.MethodInvocation visitMethodInvocation(
          J.MethodInvocation method, ExecutionContext ctx) {
        J.MethodInvocation m = super.visitMethodInvocation(method, ctx);
        LaunchDarklyCall call = LaunchDarklyCall.detect(m, getCursor());
        if (call == null || !call.rule.getId().equals(RULE_ID)) {
          return m;
        }
        if (Planner.plan(call.site).getKind() != PlanResult.Kind.REWRITE) {
          return m;
        }
        J.MethodInvocation replaced =
            template.apply(
                getCursor(),
                m.getCoordinates().replace(),
                ClientBindingProof.copyOf(call.client),
                bare(call.key),
                bare(call.defaultArg),
                bare(call.contextKey));
        maybeAddImport(IMPORT_CONTEXT);
        maybeRemoveImport("com.launchdarkly.sdk.LDContext");
        return replaced;
      }
    };
  }

  /** The original argument tree without its leading space; its own text is untouched. */
  private static Expression bare(Expression e) {
    return e.withPrefix(Space.EMPTY);
  }
}
