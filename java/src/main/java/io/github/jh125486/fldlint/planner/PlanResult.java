package io.github.jh125486.fldlint.planner;

/**
 * The planner outcome for one call site: rewrite, skip (with a call-site category) or unfixable,
 * mirroring go/internal/planner and the JS planner. Java edits are made by the recipe with {@code
 * JavaTemplate}, so a rewrite carries no text edit.
 */
public final class PlanResult {
  /** Outcome class. */
  public enum Kind {
    REWRITE,
    SKIP,
    /**
     * Rewritable, but a comment inside the call has no home in the rewrite: reported with no fix
     * (issue #31). Not a skip category.
     */
    UNFIXABLE
  }

  private final Kind kind;
  private final String category;
  private final String ruleId;

  private PlanResult(Kind kind, String category, String ruleId) {
    this.kind = kind;
    this.category = category;
    this.ruleId = ruleId;
  }

  public static PlanResult rewrite(String ruleId) {
    return new PlanResult(Kind.REWRITE, "", ruleId);
  }

  public static PlanResult skip(String category, String ruleId) {
    return new PlanResult(Kind.SKIP, category, ruleId);
  }

  public static PlanResult unfixable(String ruleId) {
    return new PlanResult(Kind.UNFIXABLE, "", ruleId);
  }

  public Kind getKind() {
    return kind;
  }

  /** The skip category; empty unless the kind is {@link Kind#SKIP}. */
  public String getCategory() {
    return category;
  }

  /** The matched rule id. */
  public String getRuleId() {
    return ruleId;
  }
}
