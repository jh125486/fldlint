package io.github.jh125486.fldlint;

import io.github.jh125486.fldlint.planner.CallSite;
import io.github.jh125486.fldlint.planner.Planner;
import io.github.jh125486.fldlint.spec.Binding;
import io.github.jh125486.fldlint.spec.Rule;
import io.github.jh125486.fldlint.spec.RulesGen;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.openrewrite.Cursor;
import org.openrewrite.java.MethodMatcher;
import org.openrewrite.java.tree.Expression;
import org.openrewrite.java.tree.J;
import org.openrewrite.java.tree.JRightPadded;
import org.openrewrite.java.tree.JavaType;
import org.openrewrite.java.tree.Space;
import org.openrewrite.java.tree.TypeUtils;

/**
 * Detection: turns a {@code J.MethodInvocation} into the plain {@link CallSite} the {@link Planner}
 * consumes, plus the original tree elements the rewrite substitutes. Both recipes call {@link
 * #detect}; neither re-implements it.
 */
final class LaunchDarklyCall {
  static final String RECEIVER = "com.launchdarkly.sdk.server.interfaces.LDClientInterface";
  private static final String LD_USER = "com.launchdarkly.sdk.LDUser";
  private static final MethodMatcher CONTEXT_CREATE =
      new MethodMatcher("com.launchdarkly.sdk.LDContext create(java.lang.String)", false);
  private static final Map<String, MethodMatcher> MATCHERS = new HashMap<String, MethodMatcher>();

  static {
    for (Rule rule : RulesGen.RULES) {
      Binding binding = rule.getLanguages().get("java");
      if (binding != null) {
        for (String method : binding.getMethods()) {
          MATCHERS.put(method, new MethodMatcher(RECEIVER + " " + method + "(..)", true));
        }
      }
    }
  }

  final Rule rule;
  final CallSite site;

  /** The static key literal, or null. */
  final J.Literal key;

  /** The default argument. */
  final Expression defaultArg;

  /** The argument of {@code LDContext.create(..)}, or null unless the context is simple. */
  final Expression contextKey;

  /** The proven client's declaration name, or null. */
  final J.Identifier client;

  private LaunchDarklyCall(
      Rule rule,
      CallSite site,
      J.Literal key,
      Expression defaultArg,
      Expression contextKey,
      J.Identifier client) {
    this.rule = rule;
    this.site = site;
    this.key = key;
    this.defaultArg = defaultArg;
    this.contextKey = contextKey;
    this.client = client;
  }

  /**
   * Detects a recognized SDK call, or returns null: not a call on an {@code LDClientInterface} with
   * a Java rule, no method type (unresolved), or an argument count outside the binding's range
   * (spec M0.10).
   *
   * @param m the invocation
   * @param cursor the cursor of the invocation, for scope walking and printing
   */
  static LaunchDarklyCall detect(J.MethodInvocation m, Cursor cursor) {
    Rule rule = RulesGen.lookupJavaMethod(m.getSimpleName());
    if (rule == null || m.getMethodType() == null) {
      return null;
    }
    MethodMatcher matcher = MATCHERS.get(m.getSimpleName());
    if (matcher == null || !matcher.matches(m)) {
      return null;
    }
    Binding binding = rule.getLanguages().get("java");
    List<Expression> args = m.getArguments();
    int argCount = args.size() == 1 && args.get(0) instanceof J.Empty ? 0 : args.size();
    if (!Planner.recognizesArgCount(rule, binding, argCount)) {
      return null;
    }

    CallSite.Builder site = CallSite.builder(rule.getId());
    // The static key is the finding's fingerprint identity (fingerprint.md), so every per-flag
    // rule reads it, report-only ones included. A rule with no keyArg (all-flags-state) is keyless.
    J.Literal key = null;
    if (rule.getKeyArg() >= 0) {
      Expression keyArg = args.get(rule.getKeyArg());
      if (keyArg instanceof J.Literal
          && ((J.Literal) keyArg).getType() == JavaType.Primitive.String
          && ((J.Literal) keyArg).getValue() instanceof String) {
        key = (J.Literal) keyArg;
        site.keyStatic(true);
      }
    }
    // Report-only rules (detail, bulk) need no other proof: the planner decides on the rule alone.
    if (!rule.getSkip().isEmpty()) {
      return new LaunchDarklyCall(rule, site.build(), key, null, null, null);
    }

    Expression contextArg = args.get(rule.getContextArg());
    Expression contextKey = null;
    if (contextArg instanceof J.MethodInvocation
        && CONTEXT_CREATE.matches((J.MethodInvocation) contextArg)
        && isTypeQualifier(((J.MethodInvocation) contextArg).getSelect())
        && ((J.MethodInvocation) contextArg).getArguments().size() == 1) {
      contextKey = ((J.MethodInvocation) contextArg).getArguments().get(0);
      site.contextShape(CallSite.ContextShape.SIMPLE);
    } else if (isLdUserParameter(m, rule.getContextArg())) {
      site.contextShape(CallSite.ContextShape.LD_USER);
    }

    Expression defaultArg = args.get(rule.getDefaultArg());
    site.defaultProven(defaultMatches(rule.getValueType(), defaultArg.getType()));

    J.Identifier client = ClientBindingProof.prove(cursor);
    site.clientBound(client != null && !importedNameClash(cursor));
    site.receiverPure(isPure(m.getSelect()));
    site.commentsUnhomed(hasUnhomedComment(m, cursor, contextArg));

    return new LaunchDarklyCall(rule, site.build(), key, defaultArg, contextKey, client);
  }

  /**
   * The rewrite writes {@code new ImmutableContext(..)} and imports {@code
   * dev.openfeature.sdk.ImmutableContext}. Another type of that simple name in the file (imported,
   * declared in it, or a type parameter) would capture the name, so the call is not rewritten. A
   * same-package type in another file is not visible here.
   */
  private static boolean importedNameClash(Cursor cursor) {
    J.CompilationUnit cu = cursor.firstEnclosing(J.CompilationUnit.class);
    if (cu == null) {
      return true;
    }
    for (J.Import imp : cu.getImports()) {
      if ("ImmutableContext".equals(imp.getQualid().getSimpleName())
          && !"dev.openfeature.sdk.ImmutableContext".equals(imp.getTypeName())) {
        return true;
      }
    }
    final boolean[] declared = {false};
    final java.util.Set<String> declaredHere = new java.util.HashSet<String>();
    new org.openrewrite.java.JavaIsoVisitor<Integer>() {
      @Override
      public J.ClassDeclaration visitClassDeclaration(J.ClassDeclaration cd, Integer p) {
        if ("ImmutableContext".equals(cd.getSimpleName())) {
          declared[0] = true;
        }
        if (cd.getType() != null) {
          declaredHere.add(cd.getType().getFullyQualifiedName());
        }
        return super.visitClassDeclaration(cd, p);
      }

      @Override
      public J.TypeParameter visitTypeParameter(J.TypeParameter tp, Integer p) {
        // Class and method type parameters: conservatively any in the file.
        if (tp.getName() instanceof J.Identifier
            && "ImmutableContext".equals(((J.Identifier) tp.getName()).getSimpleName())) {
          declared[0] = true;
        }
        return super.visitTypeParameter(tp, p);
      }
    }.visit(cu, 0);
    return declared[0] || inheritsUnknownMemberTypes(cursor, declaredHere);
  }

  /**
   * A class inherits the member types of its supertypes, and the type model does not list them. A
   * supertype from outside the JDK and outside this file might declare a member type named {@code
   * ImmutableContext}, so any enclosing class with such a supertype, or an unresolved one, counts
   * as a clash.
   */
  private static boolean inheritsUnknownMemberTypes(Cursor cursor, java.util.Set<String> here) {
    for (Cursor c = cursor; c != null; c = c.getParent()) {
      Object v = c.getValue();
      JavaType own = null;
      if (v instanceof J.ClassDeclaration) {
        own = ((J.ClassDeclaration) v).getType();
      } else if (v instanceof J.NewClass && ((J.NewClass) v).getBody() != null) {
        own = ((J.NewClass) v).getType();
      } else {
        continue;
      }
      if (!(own instanceof JavaType.FullyQualified) || own instanceof JavaType.Unknown) {
        return true;
      }
      java.util.List<JavaType.FullyQualified> work =
          new java.util.ArrayList<JavaType.FullyQualified>();
      java.util.Set<String> seen = new java.util.HashSet<String>();
      work.add((JavaType.FullyQualified) own);
      boolean first = true;
      while (!work.isEmpty()) {
        JavaType.FullyQualified t = work.remove(work.size() - 1);
        if (t instanceof JavaType.Unknown) {
          return true;
        }
        String fqn = t.getFullyQualifiedName();
        if (!first) {
          if (fqn.startsWith("java.")) {
            continue;
          }
          if (!here.contains(fqn)) {
            return true;
          }
        }
        first = false;
        if (!seen.add(fqn)) {
          continue;
        }
        if (t.getSupertype() != null) {
          work.add(t.getSupertype());
        }
        work.addAll(t.getInterfaces());
      }
    }
    return false;
  }

  /**
   * The receiver of a static call is absent (static import) or a type name, possibly
   * package-qualified. A variable or any expression is not: the rewrite drops the receiver, so it
   * must have no side effect.
   */
  private static boolean isTypeQualifier(Expression select) {
    if (select == null) {
      return true;
    }
    if (select instanceof J.Identifier) {
      return ((J.Identifier) select).getFieldType() == null;
    }
    if (select instanceof J.FieldAccess) {
      J.FieldAccess fa = (J.FieldAccess) select;
      return fa.getName().getFieldType() == null && isTypeQualifier(fa.getTarget());
    }
    return false;
  }

  private static boolean isLdUserParameter(J.MethodInvocation m, int index) {
    List<JavaType> params = m.getMethodType().getParameterTypes();
    if (index >= params.size()) {
      return false;
    }
    JavaType t = params.get(index);
    return t instanceof JavaType.FullyQualified
        && LD_USER.equals(((JavaType.FullyQualified) t).getFullyQualifiedName());
  }

  /** The default is proven by its static type, never by its value (ADR 010 item 5). */
  private static boolean defaultMatches(String valueType, JavaType type) {
    if (type == null) {
      return false;
    }
    switch (valueType) {
      case "boolean":
        return type == JavaType.Primitive.Boolean;
      case "integer":
        return type == JavaType.Primitive.Int;
      case "number":
        return type == JavaType.Primitive.Double;
      case "string":
        // A null literal is fine: getStringValue(String, String, ..) accepts null.
        return TypeUtils.isString(type) || type == JavaType.Primitive.Null;
      default:
        return false;
    }
  }

  /**
   * An implicit {@code this}, an identifier, or a field-access chain of identifiers: dropping it
   * from the call loses no side effect (ADR 010).
   */
  private static boolean isPure(Expression select) {
    if (select == null || select instanceof J.Identifier) {
      return true;
    }
    if (select instanceof J.FieldAccess) {
      return isPure(((J.FieldAccess) select).getTarget());
    }
    return false;
  }

  /**
   * True when a comment inside the call lies outside the text of the arguments that are copied
   * verbatim: the receiver, the dots, the parentheses, the separators, an argument's own leading
   * space, and the parts of {@code LDContext.create(..)} around its key. Such a comment has no home
   * in the rewritten call (issue #31), so the call is reported with no fix.
   */
  private static boolean hasUnhomedComment(
      J.MethodInvocation m, Cursor cursor, Expression contextArg) {
    if (m.getSelect() != null) {
      // The receiver disappears with its comments.
      Cursor parent = cursor.getParent() == null ? cursor : cursor.getParent();
      String printed = m.getSelect().printTrimmed(parent);
      if (printed.contains("/*") || printed.contains("//")) {
        return true;
      }
      if (hasComment(m.getPadding().getSelect().getAfter())) {
        return true;
      }
    }
    if (hasComment(m.getName().getPrefix())
        || hasComment(m.getPadding().getArguments().getBefore())) {
      return true;
    }
    if (outsideArgumentText(m.getPadding().getArguments().getPadding().getElements())) {
      return true;
    }
    if (contextArg instanceof J.MethodInvocation
        && CONTEXT_CREATE.matches((J.MethodInvocation) contextArg)
        && isTypeQualifier(((J.MethodInvocation) contextArg).getSelect())) {
      J.MethodInvocation create = (J.MethodInvocation) contextArg;
      if (create.getSelect() != null) {
        Cursor parent = cursor.getParent() == null ? cursor : cursor.getParent();
        String printed = create.getSelect().printTrimmed(parent);
        if (printed.contains("/*")
            || printed.contains("//")
            || hasComment(create.getPadding().getSelect().getAfter())) {
          return true;
        }
      }
      return hasComment(create.getName().getPrefix())
          || hasComment(create.getPadding().getArguments().getBefore())
          || outsideArgumentText(create.getPadding().getArguments().getPadding().getElements());
    }
    return false;
  }

  private static boolean outsideArgumentText(List<JRightPadded<Expression>> elements) {
    for (JRightPadded<Expression> element : elements) {
      if (hasComment(element.getElement().getPrefix()) || hasComment(element.getAfter())) {
        return true;
      }
    }
    return false;
  }

  private static boolean hasComment(Space space) {
    return !space.getComments().isEmpty();
  }
}
