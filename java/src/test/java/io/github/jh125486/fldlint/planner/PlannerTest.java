package io.github.jh125486.fldlint.planner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

import org.junit.jupiter.api.Test;

class PlannerTest {

  @Test
  void knownMethodIsSkippedAsUnboundClientWithItsRuleId() {
    PlanResult result = Planner.plan("boolVariation");
    assertEquals(PlanResult.Kind.SKIP, result.getKind());
    assertEquals("unbound-client", result.getCategory());
    assertEquals("bool-variation", result.getRuleId());
    assertNull(result.getEdit());
  }

  @Test
  void unknownMethodIsSkippedWithAnEmptyRuleId() {
    PlanResult result = Planner.plan("noSuchMethod");
    assertEquals(PlanResult.Kind.SKIP, result.getKind());
    assertEquals("unbound-client", result.getCategory());
    assertEquals("", result.getRuleId());
  }
}
