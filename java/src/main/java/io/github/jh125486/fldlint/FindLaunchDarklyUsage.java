package io.github.jh125486.fldlint;

import io.github.jh125486.fldlint.planner.PlanResult;
import io.github.jh125486.fldlint.planner.Planner;
import io.github.jh125486.fldlint.spec.RulesGen;
import io.github.jh125486.fldlint.spec.SkipCategory;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.openrewrite.ExecutionContext;
import org.openrewrite.Recipe;
import org.openrewrite.TreeVisitor;
import org.openrewrite.java.JavaIsoVisitor;
import org.openrewrite.java.tree.J;
import org.openrewrite.marker.SearchResult;

/**
 * Reports every LaunchDarkly flag call: marks it with a {@link SearchResult} and adds one {@link
 * LaunchDarklyUsage} row. It is {@code dryRun}-only: {@code run} would write the markers into
 * source (ADR 009 D5). It calls the same {@link Planner} as the rewrite recipe.
 */
public class FindLaunchDarklyUsage extends Recipe {
  // The data table registers itself with the recipe; the table never reads the recipe back.
  @SuppressWarnings("this-escape")
  private final transient LaunchDarklyUsage usage = new LaunchDarklyUsage(this);

  @Override
  public String getDisplayName() {
    return "Find LaunchDarkly flag calls";
  }

  @Override
  public String getDescription() {
    return "Marks each LaunchDarkly Java SDK flag call and reports whether it can be migrated to"
        + " OpenFeature, with the skip category when it cannot. Use with dryRun; run writes the"
        + " markers into source.";
  }

  @Override
  public TreeVisitor<?, ExecutionContext> getVisitor() {
    return new JavaIsoVisitor<ExecutionContext>() {
      // A finding collected while visiting one file.
      final class Found {
        final UUID id;
        final LaunchDarklyCall call;
        final PlanResult plan;

        Found(UUID id, LaunchDarklyCall call, PlanResult plan) {
          this.id = id;
          this.call = call;
          this.plan = plan;
        }
      }

      private final List<Found> found = new ArrayList<Found>();

      @Override
      public J.CompilationUnit visitCompilationUnit(J.CompilationUnit cu, ExecutionContext ctx) {
        found.clear();
        J.CompilationUnit result = super.visitCompilationUnit(cu, ctx);
        Set<UUID> ids = new HashSet<UUID>();
        for (Found f : found) {
          ids.add(f.id);
        }
        Map<UUID, int[]> positions = SourcePositions.of(cu, ids);
        String path = cu.getSourcePath().toString();
        List<Fingerprint.Positioned> inputs = new ArrayList<Fingerprint.Positioned>();
        for (Found f : found) {
          int[] pos = positions.get(f.id);
          inputs.add(
              new Fingerprint.Positioned(f.plan.getRuleId(), keyValue(f.call), pos[0], pos[1]));
        }
        int[] occ = Fingerprint.assignOccurrences(inputs);
        for (int i = 0; i < found.size(); i++) {
          Found f = found.get(i);
          int[] pos = positions.get(f.id);
          usage.insertRow(
              ctx,
              new LaunchDarklyUsage.Row(
                  path,
                  pos[0],
                  pos[1],
                  f.plan.getRuleId(),
                  f.plan.getCategory(),
                  f.plan.getKind() == PlanResult.Kind.UNFIXABLE ? "comment" : "",
                  message(f.call, f.plan),
                  Fingerprint.of(f.plan.getRuleId(), path, keyValue(f.call), occ[i])));
        }
        return result;
      }

      @Override
      public J.MethodInvocation visitMethodInvocation(
          J.MethodInvocation method, ExecutionContext ctx) {
        J.MethodInvocation m = super.visitMethodInvocation(method, ctx);
        LaunchDarklyCall call = LaunchDarklyCall.detect(m, getCursor());
        if (call == null) {
          return m;
        }
        PlanResult plan = Planner.plan(call.site);
        found.add(new Found(m.getId(), call, plan));
        return SearchResult.found(m, message(call, plan));
      }
    };
  }

  private static String keyValue(LaunchDarklyCall call) {
    return call.key == null ? null : (String) call.key.getValue();
  }

  /** The diagnostic text of output-format.md: rule message, or category message plus suffix. */
  static String message(LaunchDarklyCall call, PlanResult plan) {
    switch (plan.getKind()) {
      case SKIP:
        SkipCategory category = RulesGen.SKIP_CATEGORIES.get(plan.getCategory());
        return category.getMessage() + " [skip: " + plan.getCategory() + "]";
      case UNFIXABLE:
        return ruleMessage(call) + " [no-fix: comment]";
      default:
        return ruleMessage(call);
    }
  }

  private static String ruleMessage(LaunchDarklyCall call) {
    String key = call.key == null ? "" : call.key.getValueSource();
    return call.rule.getMessage().replace("{{key}}", key == null ? "" : key);
  }
}
