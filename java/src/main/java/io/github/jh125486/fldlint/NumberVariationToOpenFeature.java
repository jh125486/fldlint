package io.github.jh125486.fldlint;

/**
 * Rewrites {@code doubleVariation} (rule {@code number-variation}) to {@code
 * getDoubleValue(..).doubleValue()}.
 */
public class NumberVariationToOpenFeature extends VariationToOpenFeature {
  /** Creates the recipe. */
  public NumberVariationToOpenFeature() {
    super("number-variation");
  }

  @Override
  public String getDisplayName() {
    return "Migrate LaunchDarkly doubleVariation to OpenFeature";
  }

  @Override
  public String getDescription() {
    return describe("doubleVariation", "getDoubleValue", "a double");
  }
}
