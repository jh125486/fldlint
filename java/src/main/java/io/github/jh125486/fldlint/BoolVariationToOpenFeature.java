package io.github.jh125486.fldlint;

/**
 * Rewrites {@code boolVariation} (rule {@code bool-variation}) to {@code
 * getBooleanValue(..).booleanValue()}.
 */
public class BoolVariationToOpenFeature extends VariationToOpenFeature {
  /** Creates the recipe. */
  public BoolVariationToOpenFeature() {
    super("bool-variation");
  }

  @Override
  public String getDisplayName() {
    return "Migrate LaunchDarkly boolVariation to OpenFeature";
  }

  @Override
  public String getDescription() {
    return describe("boolVariation", "getBooleanValue", "a boolean");
  }
}
