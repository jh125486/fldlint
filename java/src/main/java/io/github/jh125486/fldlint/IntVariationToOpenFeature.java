package io.github.jh125486.fldlint;

/**
 * Rewrites {@code intVariation} (rule {@code java-int-variation}) to {@code
 * getIntegerValue(..).intValue()}.
 */
public class IntVariationToOpenFeature extends VariationToOpenFeature {
  /** Creates the recipe. */
  public IntVariationToOpenFeature() {
    super("java-int-variation");
  }

  @Override
  public String getDisplayName() {
    return "Migrate LaunchDarkly intVariation to OpenFeature";
  }

  @Override
  public String getDescription() {
    return describe("intVariation", "getIntegerValue", "an int");
  }
}
