package io.github.jh125486.fldlint;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import org.openrewrite.Cursor;
import org.openrewrite.Tree;
import org.openrewrite.java.JavaIsoVisitor;
import org.openrewrite.java.tree.Expression;
import org.openrewrite.java.tree.J;
import org.openrewrite.java.tree.JavaType;
import org.openrewrite.java.tree.Space;
import org.openrewrite.java.tree.TypeUtils;

/**
 * Proves, from the cursor of a call, that exactly one OpenFeature {@code Client} is in scope under
 * its simple name (ADR 009): the innermost scope that declares any accessible {@code Client} must
 * declare exactly one, no inner declaration of the same name may shadow it, a local captured by a
 * lambda or a local or anonymous class must be effectively final, and an instance field is not
 * accessible from a static context. Anything else is {@code unbound-client}.
 */
final class ClientBindingProof {
  static final String CLIENT_TYPE = "dev.openfeature.sdk.Client";

  private ClientBindingProof() {}

  /** One variable visible at some scope level. */
  private static final class Decl {
    final String name;
    final J.Identifier ident;
    final boolean client;
    final boolean field;
    final boolean staticField;
    final boolean captured;
    final boolean finalModifier;
    final boolean hasInitializerOrIsParam;

    Decl(
        String name,
        J.Identifier ident,
        boolean client,
        boolean field,
        boolean staticField,
        boolean captured,
        boolean finalModifier,
        boolean hasInitializerOrIsParam) {
      this.name = name;
      this.ident = ident;
      this.client = client;
      this.field = field;
      this.staticField = staticField;
      this.captured = captured;
      this.finalModifier = finalModifier;
      this.hasInitializerOrIsParam = hasInitializerOrIsParam;
    }
  }

  /**
   * The identifier of the proven client, or null when no single client is proven.
   *
   * @param start the cursor of the call (its value is the call, or the node it replaced)
   */
  static J.Identifier prove(Cursor start) {
    Set<String> shadowed = new HashSet<String>();
    boolean staticSoFar = false;
    boolean captured = false;
    Object child = null;
    for (Cursor c = treeCursor(start); c != null; c = treeCursor(c.getParent())) {
      Object v = c.getValue();
      Cursor parent = treeCursor(c.getParent());
      Object parentValue = parent == null ? null : parent.getValue();
      List<Decl> level = new ArrayList<Decl>();
      boolean crossesClass = false;
      boolean crossesLambda = false;
      if (v instanceof J.Block) {
        J.Block block = (J.Block) v;
        boolean classBody =
            (parentValue instanceof J.ClassDeclaration
                    && block.equals(((J.ClassDeclaration) parentValue).getBody()))
                || (parentValue instanceof J.NewClass
                    && block.equals(((J.NewClass) parentValue).getBody()));
        if (classBody) {
          boolean isInterface =
              parentValue instanceof J.ClassDeclaration
                  && ((J.ClassDeclaration) parentValue).getKind()
                      == J.ClassDeclaration.Kind.Type.Interface;
          staticSoFar |= memberIsStatic(child);
          for (org.openrewrite.java.tree.Statement s : block.getStatements()) {
            if (s instanceof J.VariableDeclarations) {
              addVariables(level, (J.VariableDeclarations) s, true, isInterface, captured, false);
            }
          }
          crossesClass = true;
        } else {
          List<org.openrewrite.java.tree.Statement> statements = block.getStatements();
          int idx = child == null ? -1 : indexOf(statements, child);
          for (int i = 0; i < idx; i++) {
            if (statements.get(i) instanceof J.VariableDeclarations) {
              addVariables(
                  level, (J.VariableDeclarations) statements.get(i), false, false, captured, false);
            }
          }
        }
      } else if (v instanceof J.MethodDeclaration) {
        for (org.openrewrite.java.tree.Statement p : ((J.MethodDeclaration) v).getParameters()) {
          if (p instanceof J.VariableDeclarations) {
            addVariables(level, (J.VariableDeclarations) p, false, false, captured, true);
          }
        }
      } else if (v instanceof J.Lambda) {
        for (J p : ((J.Lambda) v).getParameters().getParameters()) {
          if (p instanceof J.VariableDeclarations) {
            addVariables(level, (J.VariableDeclarations) p, false, false, captured, true);
          } else if (p instanceof J.Identifier) {
            J.Identifier id = (J.Identifier) p;
            level.add(
                new Decl(
                    id.getSimpleName(),
                    id,
                    isClient(id.getType()),
                    false,
                    false,
                    captured,
                    false,
                    true));
          }
        }
        crossesLambda = true;
      } else if (v instanceof J.ForLoop) {
        for (org.openrewrite.java.tree.Statement s : ((J.ForLoop) v).getControl().getInit()) {
          if (s instanceof J.VariableDeclarations) {
            addVariables(level, (J.VariableDeclarations) s, false, false, captured, false);
          }
        }
      } else if (v instanceof J.ForEachLoop) {
        org.openrewrite.java.tree.Statement var = ((J.ForEachLoop) v).getControl().getVariable();
        if (var instanceof J.VariableDeclarations) {
          addVariables(level, (J.VariableDeclarations) var, false, false, captured, true);
        }
      } else if (v instanceof J.Try) {
        if (((J.Try) v).getResources() != null) {
          for (J.Try.Resource r : ((J.Try) v).getResources()) {
            if (r.getVariableDeclarations() instanceof J.VariableDeclarations) {
              addVariables(
                  level,
                  (J.VariableDeclarations) r.getVariableDeclarations(),
                  false,
                  false,
                  captured,
                  false);
            }
          }
        }
      } else if (v instanceof J.Try.Catch) {
        addVariables(
            level, ((J.Try.Catch) v).getParameter().getTree(), false, false, captured, true);
      }

      if (!level.isEmpty()) {
        List<Decl> accessible = new ArrayList<Decl>();
        for (Decl d : level) {
          if (d.client && (!d.field || !staticSoFar || d.staticField)) {
            accessible.add(d);
          }
        }
        if (accessible.size() > 1) {
          return null;
        }
        if (accessible.size() == 1) {
          Decl d = accessible.get(0);
          if (shadowed.contains(d.name)) {
            return null;
          }
          J.CompilationUnit cu = c.firstEnclosing(J.CompilationUnit.class);
          if (cu == null) {
            return null;
          }
          if (patternBindingNames(cu).contains(d.name)) {
            return null;
          }
          if (!d.field && d.captured && !d.finalModifier) {
            if (!d.hasInitializerOrIsParam || isReassigned(cu, d.name)) {
              return null;
            }
          }
          return d.ident;
        }
        for (Decl d : level) {
          shadowed.add(d.name);
        }
      }
      if (crossesClass || crossesLambda) {
        captured = true;
      }
      child = v;
    }
    return null;
  }

  private static Cursor treeCursor(Cursor c) {
    Cursor cur = c;
    while (cur != null && !(cur.getValue() instanceof Tree)) {
      cur = cur.getParent();
    }
    return cur;
  }

  private static int indexOf(List<org.openrewrite.java.tree.Statement> statements, Object child) {
    for (int i = 0; i < statements.size(); i++) {
      if (statements.get(i).equals(child)) {
        return i;
      }
    }
    return -1;
  }

  /** Whether the class member on the path to the call is static (or a static nested type). */
  private static boolean memberIsStatic(Object member) {
    if (member instanceof J.MethodDeclaration) {
      return ((J.MethodDeclaration) member).hasModifier(J.Modifier.Type.Static);
    }
    if (member instanceof J.VariableDeclarations) {
      return ((J.VariableDeclarations) member).hasModifier(J.Modifier.Type.Static);
    }
    if (member instanceof J.Block) {
      return ((J.Block) member).isStatic();
    }
    if (member instanceof J.ClassDeclaration) {
      J.ClassDeclaration cd = (J.ClassDeclaration) member;
      return cd.hasModifier(J.Modifier.Type.Static)
          || cd.getKind() != J.ClassDeclaration.Kind.Type.Class;
    }
    return false;
  }

  private static void addVariables(
      List<Decl> level,
      J.VariableDeclarations vd,
      boolean field,
      boolean interfaceMember,
      boolean captured,
      boolean parameter) {
    boolean isStatic = interfaceMember || vd.hasModifier(J.Modifier.Type.Static);
    boolean isFinal = vd.hasModifier(J.Modifier.Type.Final);
    for (J.VariableDeclarations.NamedVariable nv : vd.getVariables()) {
      JavaType type = nv.getVariableType() == null ? nv.getType() : nv.getVariableType().getType();
      level.add(
          new Decl(
              nv.getSimpleName(),
              nv.getName(),
              isClient(type),
              field,
              field && isStatic,
              captured,
              isFinal,
              parameter || nv.getInitializer() != null));
    }
  }

  private static boolean isClient(JavaType type) {
    return type != null && TypeUtils.isAssignableTo(CLIENT_TYPE, type);
  }

  /** Names bound by instanceof patterns and switch case labels anywhere in the file. */
  private static Set<String> patternBindingNames(J.CompilationUnit cu) {
    final Set<String> names = new HashSet<String>();
    new JavaIsoVisitor<Set<String>>() {
      @Override
      public J.InstanceOf visitInstanceOf(J.InstanceOf instanceOf, Set<String> acc) {
        if (instanceOf.getPattern() != null) {
          collectIdentifiers(instanceOf.getPattern(), acc);
        }
        return super.visitInstanceOf(instanceOf, acc);
      }

      @Override
      public J.Case visitCase(J.Case c, Set<String> acc) {
        for (J label : c.getCaseLabels()) {
          if (!(label instanceof J.Literal)) {
            collectIdentifiers(label, acc);
          }
        }
        return super.visitCase(c, acc);
      }
    }.visit(cu, names);
    return names;
  }

  private static void collectIdentifiers(J tree, Set<String> acc) {
    new JavaIsoVisitor<Set<String>>() {
      @Override
      public J.Identifier visitIdentifier(J.Identifier identifier, Set<String> names) {
        names.add(identifier.getSimpleName());
        return identifier;
      }
    }.visit(tree, acc);
  }

  /** Conservative: any assignment, compound assignment, ++ or -- to this name anywhere. */
  private static boolean isReassigned(J.CompilationUnit cu, String name) {
    final boolean[] found = {false};
    new JavaIsoVisitor<Integer>() {
      private boolean isName(Expression e) {
        return e instanceof J.Identifier && ((J.Identifier) e).getSimpleName().equals(name);
      }

      @Override
      public J.Assignment visitAssignment(J.Assignment a, Integer p) {
        if (isName(a.getVariable())) {
          found[0] = true;
        }
        return super.visitAssignment(a, p);
      }

      @Override
      public J.AssignmentOperation visitAssignmentOperation(J.AssignmentOperation a, Integer p) {
        if (isName(a.getVariable())) {
          found[0] = true;
        }
        return super.visitAssignmentOperation(a, p);
      }

      @Override
      public J.Unary visitUnary(J.Unary u, Integer p) {
        if (isName(u.getExpression()) && u.getOperator().isModifying()) {
          found[0] = true;
        }
        return super.visitUnary(u, p);
      }
    }.visit(cu, 0);
    return found[0];
  }

  /** A copy of a declaration's name, safe to place into another tree. */
  static J.Identifier copyOf(J.Identifier ident) {
    return ident.withId(Tree.randomId()).withPrefix(Space.EMPTY);
  }
}
