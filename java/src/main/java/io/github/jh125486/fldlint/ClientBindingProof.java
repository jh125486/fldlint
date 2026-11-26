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

    /** In scope for shadowing, but never a binding (not yet initialized, or not yet declared). */
    boolean noCandidate;

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
    // Inside explicit this(..)/super(..) arguments: no instance fields, but parameters are fine.
    boolean noInstanceFields = false;
    boolean captured = false;
    Object child = null;
    for (Cursor c = treeCursor(start); c != null; c = treeCursor(c.getParent())) {
      Object v = c.getValue();
      Cursor parent = treeCursor(c.getParent());
      Object parentValue = parent == null ? null : parent.getValue();
      List<Decl> level = new ArrayList<Decl>();
      if (v instanceof J.ClassDeclaration
          && (((J.ClassDeclaration) v).getKind() != J.ClassDeclaration.Kind.Type.Class
              || ((J.ClassDeclaration) v).hasModifier(J.Modifier.Type.Static))) {
        // A static, local or member, class: nothing outside it that is not static is reachable.
        staticSoFar = true;
      }
      if (v instanceof J.MethodInvocation
          && child != null
          && ((J.MethodInvocation) v).getSelect() == null
          && ("super".equals(((J.MethodInvocation) v).getSimpleName())
              || "this".equals(((J.MethodInvocation) v).getSimpleName()))) {
        // Arguments of an explicit constructor call cannot reference instance fields.
        noInstanceFields = true;
      }
      boolean crossesClass = false;
      boolean crossesLambda = false;
      if (v instanceof J.Block) {
        J.Block block = (J.Block) v;
        if (isClassBody(block, parentValue)) {
          boolean isInterface =
              parentValue instanceof J.ClassDeclaration
                  && ((J.ClassDeclaration) parentValue).getKind()
                      == J.ClassDeclaration.Kind.Type.Interface;
          staticSoFar |= memberIsStatic(child);
          List<org.openrewrite.java.tree.Statement> members = block.getStatements();
          int memberIdx = child == null ? -1 : indexOf(members, child);
          // A field initializer or an initializer block may not name a field declared at or after
          // it by its simple name (JLS 8.3.3).
          boolean forwardRestricted =
              child instanceof J.VariableDeclarations
                  || child instanceof J.Block
                  || child instanceof J.EnumValueSet;
          // A blank final field may be unassigned inside a constructor, an initializer block or a
          // field initializer, where reading it is a compile error.
          boolean blankFinalUnsafe =
              (child instanceof J.MethodDeclaration
                      && ((J.MethodDeclaration) child).isConstructor())
                  || child instanceof J.Block
                  || child instanceof J.VariableDeclarations;
          for (int i = 0; i < members.size(); i++) {
            if (members.get(i) instanceof J.EnumValueSet) {
              // Enum constants are static fields: they shadow, and are never a binding.
              for (J.EnumValue ev : ((J.EnumValueSet) members.get(i)).getEnums()) {
                int before = level.size();
                level.add(
                    new Decl(
                        ev.getName().getSimpleName(),
                        ev.getName(),
                        false,
                        true,
                        true,
                        captured,
                        true,
                        true));
                markNoCandidate(level, before);
              }
            }
            if (members.get(i) instanceof J.VariableDeclarations) {
              int before = level.size();
              addVariables(
                  level,
                  (J.VariableDeclarations) members.get(i),
                  true,
                  isInterface,
                  captured,
                  false);
              if (forwardRestricted && i >= memberIdx) {
                markNoCandidate(level, before);
              }
              if (blankFinalUnsafe) {
                for (int k = before; k < level.size(); k++) {
                  Decl blank = level.get(k);
                  if (blank.finalModifier && !blank.hasInitializerOrIsParam) {
                    blank.noCandidate = true;
                  }
                }
              }
            }
          }
          if (parentValue instanceof J.ClassDeclaration
              && ((J.ClassDeclaration) parentValue).getPrimaryConstructor() != null) {
            // Record components are fields the visitor does not list as members: shadow only.
            for (org.openrewrite.java.tree.Statement rc :
                ((J.ClassDeclaration) parentValue).getPrimaryConstructor()) {
              if (rc instanceof J.VariableDeclarations) {
                int before = level.size();
                addVariables(level, (J.VariableDeclarations) rc, true, false, captured, false);
                markNoCandidate(level, before);
              }
            }
          }
          // Inherited fields hide an outer variable of the same name. Unknown supertypes cannot be
          // ruled out, so the proof gives up.
          JavaType own =
              parentValue instanceof J.ClassDeclaration
                  ? ((J.ClassDeclaration) parentValue).getType()
                  : ((J.NewClass) parentValue).getType();
          if (parentValue instanceof J.ClassDeclaration
              && !declaredSupertypesResolved((J.ClassDeclaration) parentValue)) {
            return null;
          }
          if (parentValue instanceof J.NewClass
              && ((J.NewClass) parentValue).getClazz() != null
              && (((J.NewClass) parentValue).getClazz().getType() == null
                  || ((J.NewClass) parentValue).getClazz().getType() instanceof JavaType.Unknown)) {
            return null;
          }
          Set<String> inherited = inheritedFieldNames(own);
          if (inherited == null) {
            return null;
          }
          for (String name : inherited) {
            int before = level.size();
            level.add(new Decl(name, null, false, true, false, captured, true, true));
            markNoCandidate(level, before);
          }
          crossesClass = true;
        } else {
          List<org.openrewrite.java.tree.Statement> statements = block.getStatements();
          int idx = child == null ? -1 : indexOf(statements, child);
          for (int i = 0; i < idx; i++) {
            addLocalStatement(level, statements.get(i), captured);
          }
        }
      } else if (v instanceof J.Case) {
        // A classic switch case group is not a block, but its statements share the switch scope.
        List<org.openrewrite.java.tree.Statement> statements = ((J.Case) v).getStatements();
        int idx = child == null ? -1 : indexOf(statements, child);
        for (int i = 0; i < idx; i++) {
          addLocalStatement(level, statements.get(i), captured);
        }
      } else if (v instanceof J.VariableDeclarations
          && child instanceof J.VariableDeclarations.NamedVariable
          && !(parentValue instanceof J.Block
              && isClassBody((J.Block) parentValue, grandValue(parent)))) {
        // A local is in scope inside its own initializer, and so are the declarators before it.
        J.VariableDeclarations vd = (J.VariableDeclarations) v;
        int before = level.size();
        addVariables(level, vd, false, false, captured, false);
        int upTo = indexOfVariable(vd, child);
        while (level.size() > before + upTo + 1) {
          level.remove(level.size() - 1);
        }
        markNoCandidate(level, before);
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
        int before = level.size();
        for (org.openrewrite.java.tree.Statement s : ((J.ForLoop) v).getControl().getInit()) {
          if (s instanceof J.VariableDeclarations) {
            addVariables(level, (J.VariableDeclarations) s, false, false, captured, false);
          }
        }
        if (!isBody(child, ((J.ForLoop) v).getBody())) {
          markNoCandidate(level, before);
        }
      } else if (v instanceof J.ForEachLoop) {
        org.openrewrite.java.tree.Statement var = ((J.ForEachLoop) v).getControl().getVariable();
        if (var instanceof J.VariableDeclarations) {
          addVariables(level, (J.VariableDeclarations) var, false, false, captured, true);
          if (!isBody(child, ((J.ForEachLoop) v).getBody())) {
            markNoCandidate(level, 0);
          }
        }
      } else if (v instanceof J.Try) {
        // Resources are in scope in the body and in later resources, never in catch or finally.
        J.Try tr = (J.Try) v;
        List<J.Try.Resource> resources = tr.getResources();
        if (resources != null && (isBody(child, tr.getBody()) || child instanceof J.Try.Resource)) {
          for (J.Try.Resource r : resources) {
            if (child instanceof J.Try.Resource && r.equals(child)) {
              break;
            }
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
          // A client that is in scope but not readable here (not yet declared, not yet
          // assigned, still being initialized) makes the choice unprovable: giving up beats
          // silently selecting another client.
          if (d.client
              && d.noCandidate
              && (d.field ? !(staticSoFar || noInstanceFields) || d.staticField : !staticSoFar)) {
            return null;
          }
          // Past a static boundary only static fields are reachable: not instance fields, and not
          // the
          // locals and parameters of an enclosing method either.
          if (d.client
              && !d.noCandidate
              && (d.field ? !(staticSoFar || noInstanceFields) || d.staticField : !staticSoFar)) {
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
          // A local must be definitely assigned; an initializer (or being a parameter) proves it.
          if (!d.field && !d.hasInitializerOrIsParam) {
            return null;
          }
          if (!d.field && d.captured && !d.finalModifier && isReassigned(cu, d.name)) {
            return null;
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

  /**
   * Names of the fields of the supertypes (superclasses and interfaces, transitively) of a class,
   * or null when a supertype is unknown (no type attribution), so the set cannot be trusted.
   */
  private static Set<String> inheritedFieldNames(JavaType type) {
    if (!(type instanceof JavaType.FullyQualified) || type instanceof JavaType.Unknown) {
      return null;
    }
    Set<String> names = new HashSet<String>();
    Set<String> seen = new HashSet<String>();
    List<JavaType.FullyQualified> work = new ArrayList<JavaType.FullyQualified>();
    JavaType.FullyQualified own = (JavaType.FullyQualified) type;
    if (own.getSupertype() != null) {
      work.add(own.getSupertype());
    }
    work.addAll(own.getInterfaces());
    while (!work.isEmpty()) {
      JavaType.FullyQualified t = work.remove(work.size() - 1);
      if (t instanceof JavaType.Unknown) {
        return null;
      }
      if (!seen.add(t.getFullyQualifiedName())) {
        continue;
      }
      for (JavaType.Variable f : t.getMembers()) {
        names.add(f.getName());
      }
      if (t.getSupertype() != null) {
        work.add(t.getSupertype());
      }
      work.addAll(t.getInterfaces());
    }
    return names;
  }

  private static boolean declaredSupertypesResolved(J.ClassDeclaration cd) {
    List<org.openrewrite.java.tree.TypeTree> declared =
        new ArrayList<org.openrewrite.java.tree.TypeTree>();
    if (cd.getExtends() != null) {
      declared.add(cd.getExtends());
    }
    if (cd.getImplements() != null) {
      declared.addAll(cd.getImplements());
    }
    for (org.openrewrite.java.tree.TypeTree t : declared) {
      if (t.getType() == null || t.getType() instanceof JavaType.Unknown) {
        return false;
      }
    }
    return true;
  }

  private static boolean isClassBody(J.Block block, Object parentValue) {
    return (parentValue instanceof J.ClassDeclaration
            && block.equals(((J.ClassDeclaration) parentValue).getBody()))
        || (parentValue instanceof J.NewClass
            && block.equals(((J.NewClass) parentValue).getBody()));
  }

  private static Object grandValue(Cursor parent) {
    Cursor g = parent == null ? null : treeCursor(parent.getParent());
    return g == null ? null : g.getValue();
  }

  private static boolean isBody(Object child, Object body) {
    return child != null && child.equals(body);
  }

  private static void markNoCandidate(List<Decl> level, int from) {
    for (int i = from; i < level.size(); i++) {
      level.get(i).noCandidate = true;
    }
  }

  private static int indexOfVariable(J.VariableDeclarations vd, Object named) {
    List<J.VariableDeclarations.NamedVariable> vars = vd.getVariables();
    for (int i = 0; i < vars.size(); i++) {
      if (vars.get(i).equals(named)) {
        return i;
      }
    }
    return vars.size() - 1;
  }

  /** A statement that may declare locals: a declaration, or a case group of a classic switch. */
  private static void addLocalStatement(
      List<Decl> level, org.openrewrite.java.tree.Statement s, boolean captured) {
    if (s instanceof J.VariableDeclarations) {
      addVariables(level, (J.VariableDeclarations) s, false, false, captured, false);
    } else if (s instanceof J.Case) {
      for (org.openrewrite.java.tree.Statement inner : ((J.Case) s).getStatements()) {
        if (inner instanceof J.VariableDeclarations) {
          addVariables(level, (J.VariableDeclarations) inner, false, false, captured, false);
        }
      }
    }
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
    if (member instanceof J.EnumValueSet) {
      // Enum constant arguments are evaluated statically.
      return true;
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
