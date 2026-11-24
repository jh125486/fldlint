package io.github.jh125486.fldlint.planner;

/**
 * The planner outcome for one call site: a rewrite (with an edit) or a skip (with a category),
 * mirroring go/internal/planner and the JS planner.
 */
public final class PlanResult {
  /** Outcome class. */
  public enum Kind {
    REWRITE,
    SKIP
  }

  /** A text replacement of the half-open range [start, end). */
  public static final class Edit {
    private final int start;
    private final int end;
    private final String text;

    public Edit(int start, int end, String text) {
      this.start = start;
      this.end = end;
      this.text = text;
    }

    public int getStart() {
      return start;
    }

    public int getEnd() {
      return end;
    }

    public String getText() {
      return text;
    }
  }

  private final Kind kind;
  private final String category;
  private final String ruleId;
  private final Edit edit;

  private PlanResult(Kind kind, String category, String ruleId, Edit edit) {
    this.kind = kind;
    this.category = category;
    this.ruleId = ruleId;
    this.edit = edit;
  }

  public static PlanResult rewrite(String ruleId, Edit edit) {
    return new PlanResult(Kind.REWRITE, "", ruleId, edit);
  }

  public static PlanResult skip(String category, String ruleId) {
    return new PlanResult(Kind.SKIP, category, ruleId, null);
  }

  public Kind getKind() {
    return kind;
  }

  /** The skip category; empty for a rewrite. */
  public String getCategory() {
    return category;
  }

  /** The matched rule id, or empty when the method is not bound to a rule. */
  public String getRuleId() {
    return ruleId;
  }

  /** The edit; null for a skip. */
  public Edit getEdit() {
    return edit;
  }
}
