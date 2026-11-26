package io.github.jh125486.fldlint.planner;

import io.github.jh125486.fldlint.spec.Binding;
import io.github.jh125486.fldlint.spec.Rule;
import io.github.jh125486.fldlint.spec.RulesGen;

/**
 * Decides, for one detected call site, whether the call is rewritten or only reported. Both Java
 * recipes call {@link #plan(CallSite)}; neither re-implements detection or the step order.
 *
 * <p>Step order (identical across languages, ADR 010, first match wins): rule-level skip, {@code
 * dynamic-key}, {@code context-unproven}, {@code default-type-unproven}, {@code unbound-client},
 * {@code receiver-unproven}, else rewrite (or, when a comment inside the call has no home, report
 * it without a fix).
 */
public final class Planner {
  /** Call-site skip category: the flag key is not a string literal. */
  public static final String CATEGORY_DYNAMIC_KEY = "dynamic-key";

  /** Call-site skip category: the context is not a proven {@code LDContext.create(key)}. */
  public static final String CATEGORY_CONTEXT_UNPROVEN = "context-unproven";

  /** Call-site skip category: the default's static type is not the rule's type. */
  public static final String CATEGORY_DEFAULT_TYPE_UNPROVEN = "default-type-unproven";

  /** Call-site skip category: no single OpenFeature client is proven in scope. */
  public static final String CATEGORY_UNBOUND_CLIENT = "unbound-client";

  /** Call-site skip category: removing the receiver expression is not provably safe. */
  public static final String CATEGORY_RECEIVER_UNPROVEN = "receiver-unproven";

  private Planner() {}

  /** Finds the rule with this id, or null. */
  public static Rule lookupRule(String ruleId) {
    for (Rule rule : RulesGen.RULES) {
      if (rule.getId().equals(ruleId)) {
        return rule;
      }
    }
    return null;
  }

  /**
   * Whether a call with {@code argCount} arguments is a recognized SDK call for this binding. A
   * report-only binding carries {@code argCount} (spec M0.10): a count outside the range is an
   * unrecognized call (no finding), exactly like the JS and Go engines. A rewriting binding has no
   * {@code argCount}; its calls must carry every positional argument the template reads.
   */
  public static boolean recognizesArgCount(Rule rule, Binding binding, int argCount) {
    Integer min = binding.getArgCountMin();
    if (min != null) {
      Integer max = binding.getArgCountMax();
      return argCount >= min && (max == null || argCount <= max);
    }
    int needed =
        Math.max(rule.getKeyArg(), Math.max(rule.getDefaultArg(), rule.getContextArg())) + 1;
    return argCount == needed;
  }

  /** Plans one detected LaunchDarkly call. */
  public static PlanResult plan(CallSite site) {
    Rule rule = lookupRule(site.getRuleId());
    if (rule == null) {
      throw new IllegalArgumentException("unknown rule id: " + site.getRuleId());
    }
    if (!rule.getSkip().isEmpty()) {
      return PlanResult.skip(rule.getSkip(), rule.getId());
    }
    if (!site.isKeyStatic()) {
      return PlanResult.skip(CATEGORY_DYNAMIC_KEY, rule.getId());
    }
    if (site.getContextShape() != CallSite.ContextShape.SIMPLE) {
      return PlanResult.skip(CATEGORY_CONTEXT_UNPROVEN, rule.getId());
    }
    if (!site.isDefaultProven()) {
      return PlanResult.skip(CATEGORY_DEFAULT_TYPE_UNPROVEN, rule.getId());
    }
    if (!site.isClientBound()) {
      return PlanResult.skip(CATEGORY_UNBOUND_CLIENT, rule.getId());
    }
    if (!site.isReceiverPure()) {
      return PlanResult.skip(CATEGORY_RECEIVER_UNPROVEN, rule.getId());
    }
    if (site.isCommentsUnhomed()) {
      return PlanResult.unfixable(rule.getId());
    }
    return PlanResult.rewrite(rule.getId());
  }
}
