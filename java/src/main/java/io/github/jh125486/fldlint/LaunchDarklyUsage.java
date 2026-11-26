package io.github.jh125486.fldlint;

import org.openrewrite.Column;
import org.openrewrite.DataTable;
import org.openrewrite.Recipe;

/**
 * One row per finding of {@link FindLaunchDarklyUsage}: rewritable, skipped or unfixable. Position
 * is the start of the call including its receiver, 1-based, in UTF-16 code units with tab = 1.
 */
public class LaunchDarklyUsage extends DataTable<LaunchDarklyUsage.Row> {

  public LaunchDarklyUsage(Recipe recipe) {
    super(
        recipe,
        "LaunchDarkly usage",
        "LaunchDarkly flag calls, with the call-site skip category when the call is not rewritten.");
  }

  /** A finding. {@code skip} and {@code noFix} are empty when they do not apply. */
  public static class Row {
    @Column(displayName = "Source path", description = "The file the call is in.")
    public final String sourcePath;

    @Column(displayName = "Line", description = "1-based line of the call start.")
    public final int line;

    @Column(displayName = "Column", description = "1-based UTF-16 column of the call start.")
    public final int col;

    @Column(displayName = "Rule", description = "The spec rule id.")
    public final String ruleId;

    @Column(displayName = "Skip", description = "The call-site skip category, or empty.")
    public final String skip;

    @Column(displayName = "No fix", description = "\"comment\" when a comment blocks the fix.")
    public final String noFix;

    @Column(displayName = "Message", description = "The diagnostic text.")
    public final String message;

    @Column(displayName = "Fingerprint", description = "Stable identity (fingerprint.md).")
    public final String fingerprint;

    public Row(
        String sourcePath,
        int line,
        int col,
        String ruleId,
        String skip,
        String noFix,
        String message,
        String fingerprint) {
      this.sourcePath = sourcePath;
      this.line = line;
      this.col = col;
      this.ruleId = ruleId;
      this.skip = skip;
      this.noFix = noFix;
      this.message = message;
      this.fingerprint = fingerprint;
    }
  }
}
