package io.github.jh125486.fldlint;

/** Rewrites {@code stringVariation} (rule {@code string-variation}) to {@code getStringValue}. */
public class StringVariationToOpenFeature extends VariationToOpenFeature {
  /** Creates the recipe. */
  public StringVariationToOpenFeature() {
    super("string-variation");
  }

  @Override
  public String getDisplayName() {
    return "Migrate LaunchDarkly stringVariation to OpenFeature";
  }

  @Override
  public String getDescription() {
    return describe("stringVariation", "getStringValue", "a String or null");
  }
}
