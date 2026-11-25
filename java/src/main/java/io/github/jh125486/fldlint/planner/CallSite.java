package io.github.jh125486.fldlint.planner;

/**
 * A detected LaunchDarkly call with a proven receiver type, as plain data (no OpenRewrite types),
 * so {@link Planner#plan(CallSite)} stays pure and testable. The detector in the recipe package
 * builds it; the planner never inspects a syntax tree.
 */
public final class CallSite {
  /** How the context argument looks. */
  public enum ContextShape {
    /** Exactly {@code LDContext.create(<String expr>)}: the one shape the rewrite converts. */
    SIMPLE,
    /** Anything else: a variable, a builder, a multi-kind context, another call. */
    OTHER,
    /** The v6 {@code LDUser} overload: reported as {@code context-unproven}. */
    LD_USER
  }

  private final String ruleId;
  private final boolean keyStatic;
  private final ContextShape contextShape;
  private final boolean defaultProven;
  private final boolean clientBound;
  private final boolean receiverPure;
  private final boolean commentsUnhomed;

  private CallSite(Builder b) {
    this.ruleId = b.ruleId;
    this.keyStatic = b.keyStatic;
    this.contextShape = b.contextShape;
    this.defaultProven = b.defaultProven;
    this.clientBound = b.clientBound;
    this.receiverPure = b.receiverPure;
    this.commentsUnhomed = b.commentsUnhomed;
  }

  public static Builder builder(String ruleId) {
    return new Builder(ruleId);
  }

  public String getRuleId() {
    return ruleId;
  }

  /** The flag key is a string literal (a text block counts). */
  public boolean isKeyStatic() {
    return keyStatic;
  }

  public ContextShape getContextShape() {
    return contextShape;
  }

  /** The default argument's static type is exactly the rule's primitive (or String) type. */
  public boolean isDefaultProven() {
    return defaultProven;
  }

  /** Exactly one in-scope, unshadowed OpenFeature {@code Client} (ADR 009). */
  public boolean isClientBound() {
    return clientBound;
  }

  /** The receiver is an identifier or a field-access chain, so dropping it loses no effects. */
  public boolean isReceiverPure() {
    return receiverPure;
  }

  /** A comment inside the call has no home in the rewritten call (issue #31). */
  public boolean isCommentsUnhomed() {
    return commentsUnhomed;
  }

  /** Builder; every proof defaults to "not proven". */
  public static final class Builder {
    private final String ruleId;
    private boolean keyStatic;
    private ContextShape contextShape = ContextShape.OTHER;
    private boolean defaultProven;
    private boolean clientBound;
    private boolean receiverPure;
    private boolean commentsUnhomed;

    private Builder(String ruleId) {
      this.ruleId = ruleId;
    }

    public Builder keyStatic(boolean value) {
      this.keyStatic = value;
      return this;
    }

    public Builder contextShape(ContextShape value) {
      this.contextShape = value;
      return this;
    }

    public Builder defaultProven(boolean value) {
      this.defaultProven = value;
      return this;
    }

    public Builder clientBound(boolean value) {
      this.clientBound = value;
      return this;
    }

    public Builder receiverPure(boolean value) {
      this.receiverPure = value;
      return this;
    }

    public Builder commentsUnhomed(boolean value) {
      this.commentsUnhomed = value;
      return this;
    }

    public CallSite build() {
      return new CallSite(this);
    }
  }
}
