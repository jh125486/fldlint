package io.github.jh125486.fldlint.planner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.github.jh125486.fldlint.spec.Binding;
import io.github.jh125486.fldlint.spec.Rule;
import java.util.Collections;
import org.junit.jupiter.api.Test;

class PlannerTest {
  private static final String RULE = "bool-variation";

  /** A call that passes every proof. */
  private static CallSite.Builder proven() {
    return CallSite.builder(RULE)
        .keyStatic(true)
        .contextShape(CallSite.ContextShape.SIMPLE)
        .defaultProven(true)
        .clientBound(true)
        .receiverPure(true);
  }

  private static void assertSkip(String category, CallSite site) {
    PlanResult result = Planner.plan(site);
    assertEquals(PlanResult.Kind.SKIP, result.getKind());
    assertEquals(category, result.getCategory());
    assertEquals(RULE, result.getRuleId());
  }

  @Test
  void provenCallIsRewritten() {
    PlanResult result = Planner.plan(proven().build());
    assertEquals(PlanResult.Kind.REWRITE, result.getKind());
    assertEquals("", result.getCategory());
    assertEquals(RULE, result.getRuleId());
  }

  @Test
  void dynamicKeyIsSkipped() {
    assertSkip("dynamic-key", proven().keyStatic(false).build());
  }

  @Test
  void contextThatIsNotSimpleIsUnproven() {
    assertSkip("context-unproven", proven().contextShape(CallSite.ContextShape.OTHER).build());
    assertSkip("context-unproven", proven().contextShape(CallSite.ContextShape.LD_USER).build());
  }

  @Test
  void unprovenDefaultIsSkipped() {
    assertSkip("default-type-unproven", proven().defaultProven(false).build());
  }

  @Test
  void unboundClientIsSkipped() {
    assertSkip("unbound-client", proven().clientBound(false).build());
  }

  @Test
  void impureReceiverIsSkipped() {
    assertSkip("receiver-unproven", proven().receiverPure(false).build());
  }

  @Test
  void unhomedCommentWithholdsTheFixButIsNotASkip() {
    PlanResult result = Planner.plan(proven().commentsUnhomed(true).build());
    assertEquals(PlanResult.Kind.UNFIXABLE, result.getKind());
    assertEquals("", result.getCategory());
    assertEquals(RULE, result.getRuleId());
  }

  @Test
  void firstFailingStepWins() {
    // Every proof missing: the shared order starts with dynamic-key (ADR 010).
    assertSkip("dynamic-key", CallSite.builder(RULE).build());
    // A v6 LDUser call with a dynamic key is dynamic-key, not context-unproven.
    assertSkip(
        "dynamic-key",
        proven().keyStatic(false).contextShape(CallSite.ContextShape.LD_USER).build());
    // Context before default, default before client, client before receiver.
    assertSkip(
        "context-unproven",
        proven().contextShape(CallSite.ContextShape.OTHER).defaultProven(false).build());
    assertSkip("default-type-unproven", proven().defaultProven(false).clientBound(false).build());
    assertSkip("unbound-client", proven().clientBound(false).receiverPure(false).build());
    // A skip outranks an unhomed comment.
    assertSkip("receiver-unproven", proven().receiverPure(false).commentsUnhomed(true).build());
  }

  @Test
  void reportOnlyRuleIsSkippedByItsOwnCategory() {
    PlanResult result = Planner.plan(CallSite.builder("bool-variation-detail").build());
    assertEquals(PlanResult.Kind.SKIP, result.getKind());
    assertEquals("detail", result.getCategory());
  }

  @Test
  void unknownRuleIsRejected() {
    assertThrows(
        IllegalArgumentException.class, () -> Planner.plan(CallSite.builder("nope").build()));
  }

  @Test
  void rewritingRuleNeedsEveryPositionalArgument() {
    Rule rule = Planner.lookupRule(RULE);
    Binding binding = rule.getLanguages().get("java");
    assertTrue(Planner.recognizesArgCount(rule, binding, 3));
    assertFalse(Planner.recognizesArgCount(rule, binding, 2));
    assertFalse(Planner.recognizesArgCount(rule, binding, 4));
  }

  @Test
  void reportOnlyRuleHonorsItsArgCountRange() {
    Rule rule = Planner.lookupRule("bool-variation-detail");
    Binding bounded = binding(2, 3);
    assertFalse(Planner.recognizesArgCount(rule, bounded, 1));
    assertTrue(Planner.recognizesArgCount(rule, bounded, 2));
    assertTrue(Planner.recognizesArgCount(rule, bounded, 3));
    assertFalse(Planner.recognizesArgCount(rule, bounded, 4));

    Binding variadic = binding(1, null);
    assertFalse(Planner.recognizesArgCount(rule, variadic, 0));
    assertTrue(Planner.recognizesArgCount(rule, variadic, 1));
    assertTrue(Planner.recognizesArgCount(rule, variadic, 9));
  }

  private static Binding binding(Integer min, Integer max) {
    return new Binding(
        Collections.<String>emptyList(),
        Collections.<String, Integer>emptyMap(),
        min,
        max,
        "",
        Collections.<String>emptyList(),
        Collections.<String>emptyList(),
        false);
  }
}
