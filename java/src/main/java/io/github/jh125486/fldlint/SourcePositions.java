package io.github.jh125486.fldlint;

import java.util.HashMap;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import org.openrewrite.PrintOutputCapture;
import org.openrewrite.java.JavaPrinter;
import org.openrewrite.java.tree.J;
import org.openrewrite.java.tree.Space;

/**
 * Line and column of method invocations, measured by printing the file up to the start of each
 * invocation (its receiver included). Java strings are UTF-16, so {@code length()} is the column
 * unit of spec/contracts/output-format.md#positions; a tab counts as one.
 */
final class SourcePositions {
  private SourcePositions() {}

  /** 1-based {line, col} by invocation id, for the ids in {@code ids}. */
  static Map<UUID, int[]> of(J.CompilationUnit cu, final Set<UUID> ids) {
    final Map<UUID, Integer> offsets = new HashMap<UUID, Integer>();
    PrintOutputCapture<Integer> out = new PrintOutputCapture<Integer>(0);
    new JavaPrinter<Integer>() {
      @Override
      public J visitMethodInvocation(J.MethodInvocation m, PrintOutputCapture<Integer> p) {
        if (ids.contains(m.getId())) {
          PrintOutputCapture<Integer> prefix = new PrintOutputCapture<Integer>(0);
          visitSpace(m.getPrefix(), Space.Location.METHOD_INVOCATION_PREFIX, prefix);
          offsets.put(m.getId(), p.getOut().length() + prefix.getOut().length());
        }
        return super.visitMethodInvocation(m, p);
      }
    }.visit(cu, out);

    String text = out.getOut();
    Map<UUID, int[]> positions = new HashMap<UUID, int[]>();
    for (Map.Entry<UUID, Integer> e : offsets.entrySet()) {
      int offset = e.getValue();
      int line = 1;
      int lineStart = 0;
      for (int i = 0; i < offset; i++) {
        if (text.charAt(i) == '\n') {
          line++;
          lineStart = i + 1;
        }
      }
      positions.put(e.getKey(), new int[] {line, offset - lineStart + 1});
    }
    return positions;
  }
}
