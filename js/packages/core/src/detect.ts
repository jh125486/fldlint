// SPDX-License-Identifier: MIT
// Receiver proof is derived from flaglint-js src/scanner/index.ts @b85599f
// (collectLDClients), MIT, see NOTICE. The port keys on scope-manager
// Variable objects instead of identifier names.
import { AST_NODE_TYPES, type TSESLint, type TSESTree } from "@typescript-eslint/utils";
import { findRuleByMethod } from "@fldlint/spec";
import type { CallSite, ClientBinding, ContextShape, DefaultType } from "./callsite.js";

type Scope = TSESLint.Scope.Scope;
type Variable = TSESLint.Scope.Variable;
type Definition = TSESLint.Scope.Definition;
type Reference = TSESLint.Scope.Reference;
export type ScopeManager = TSESLint.Scope.ScopeManager;
export type VisitorKeys = TSESLint.SourceCode.VisitorKeys;

const LD_PACKAGES: readonly string[] = ["@launchdarkly/node-server-sdk"];
// Legacy package: calls are reported when a skip rule covers them, never rewritten.
const LD_LEGACY_PACKAGES: readonly string[] = ["launchdarkly-node-server-sdk"];
const OPENFEATURE_PACKAGES: readonly string[] = ["@openfeature/server-sdk"];

type Node = TSESTree.Node;
type Expr = TSESTree.Node;

function isStringLiteral(n: Node | null | undefined): n is TSESTree.StringLiteral {
  return n?.type === AST_NODE_TYPES.Literal && typeof n.value === "string";
}

function propName(key: Node): string | undefined {
  if (key.type === AST_NODE_TYPES.Identifier) return key.name;
  if (isStringLiteral(key)) return key.value;
  return undefined;
}

class Detector {
  private readonly refs = new Map<Node, Reference>();
  private readonly sites: CallSite[] = [];
  // `parent` links are set by ESLint's traversal, not by the parser; build our own.
  private readonly parents = new Map<Node, Node>();

  constructor(
    private readonly ast: TSESTree.Program,
    private readonly scopes: ScopeManager,
    private readonly source: string,
    private readonly visitorKeys: VisitorKeys,
  ) {
    for (const scope of scopes.scopes) for (const ref of scope.references) this.refs.set(ref.identifier, ref);
  }

  run(): CallSite[] {
    this.link(this.ast);
    this.walk(this.ast);
    return this.sites;
  }

  private children(node: Node): Node[] {
    const out: Node[] = [];
    for (const key of this.visitorKeys[node.type] ?? []) {
      const value = (node as unknown as Record<string, unknown>)[key];
      if (Array.isArray(value)) {
        for (const v of value) if (isNode(v)) out.push(v);
      } else if (isNode(value)) {
        out.push(value);
      }
    }
    return out;
  }

  private link(node: Node): void {
    for (const c of this.children(node)) {
      this.parents.set(c, node);
      this.link(c);
    }
  }

  private walk(node: Node): void {
    if (node.type === AST_NODE_TYPES.CallExpression) this.call(node);
    for (const c of this.children(node)) this.walk(c);
  }

  // ---------- scope helpers ----------

  private resolve(id: Node): Variable | null {
    return this.refs.get(id)?.resolved ?? null;
  }

  private innermostScope(pos: number): Scope {
    let scope: Scope = this.scopes.globalScope!;
    for (;;) {
      const next: Scope | undefined = scope.childScopes.find((c) => c.block.range[0] <= pos && pos < c.block.range[1]);
      if (!next) return scope;
      scope = next;
    }
  }

  private static singleDef(v: Variable | null): Definition | undefined {
    return v?.defs.length === 1 ? v.defs[0] : undefined;
  }

  /** No writes other than the declaration's own initializer. */
  private static neverReassigned(v: Variable): boolean {
    return v.references.every((r) => !r.isWrite() || r.init === true);
  }

  // ---------- SDK export proofs ----------

  /** `require("<pkg>")` where `require` is the unshadowed global. */
  private isRequireOf(expr: Expr | null | undefined, pkgs: readonly string[]): boolean {
    if (expr?.type !== AST_NODE_TYPES.CallExpression || expr.optional) return false;
    const { callee } = expr;
    if (callee.type !== AST_NODE_TYPES.Identifier || callee.name !== "require") return false;
    // ESLint resolves configured globals (globals.node, `globals: { require }`)
    // to a global-scope Variable with no definitions; that is still the global.
    const resolved = this.resolve(callee);
    if (resolved !== null && !(resolved.scope.type === "global" && resolved.defs.length === 0)) return false;
    const [arg] = expr.arguments;
    return expr.arguments.length === 1 && isStringLiteral(arg) && pkgs.includes(arg.value);
  }

  /** Does `expr` evaluate to the module object of one of `pkgs`? */
  private isModuleObject(expr: Expr, pkgs: readonly string[]): boolean {
    if (this.isRequireOf(expr, pkgs)) return true;
    if (expr.type !== AST_NODE_TYPES.Identifier) return false;
    const v = this.resolve(expr);
    const def = Detector.singleDef(v);
    if (!def || !v) return false;
    if (def.type === "ImportBinding") {
      const spec = def.node;
      if (isTypeOnlyImport(def)) return false;
      const fromPkg = def.parent?.type === AST_NODE_TYPES.ImportDeclaration && pkgs.includes(def.parent.source.value);
      return fromPkg && (spec.type === AST_NODE_TYPES.ImportNamespaceSpecifier || spec.type === AST_NODE_TYPES.ImportDefaultSpecifier);
    }
    if (def.type === "Variable") {
      const decl = def.node;
      return decl.id === def.name && Detector.neverReassigned(v) && this.isRequireOf(decl.init, pkgs);
    }
    return false;
  }

  /** Does `expr` evaluate to the named export `exportName` of one of `pkgs`? */
  private isSdkExport(expr: Expr, pkgs: readonly string[], exportName: string): boolean {
    if (expr.type === AST_NODE_TYPES.MemberExpression) {
      return (
        !expr.computed &&
        !expr.optional &&
        expr.property.type === AST_NODE_TYPES.Identifier &&
        expr.property.name === exportName &&
        this.isModuleObject(expr.object, pkgs)
      );
    }
    if (expr.type !== AST_NODE_TYPES.Identifier) return false;
    const v = this.resolve(expr);
    const def = Detector.singleDef(v);
    if (!def || !v) return false;
    if (def.type === "ImportBinding") {
      const spec = def.node;
      if (spec.type !== AST_NODE_TYPES.ImportSpecifier || isTypeOnlyImport(def)) return false;
      if (def.parent?.type !== AST_NODE_TYPES.ImportDeclaration || !pkgs.includes(def.parent.source.value)) return false;
      return propName(spec.imported) === exportName;
    }
    if (def.type === "Variable") {
      // const { init } = require(pkg)  /  const { init: ldInit } = require(pkg)
      const decl = def.node;
      if (decl.id.type !== AST_NODE_TYPES.ObjectPattern || !this.isRequireOf(decl.init, pkgs)) return false;
      if (!Detector.neverReassigned(v)) return false;
      return decl.id.properties.some(
        (p) => p.type === AST_NODE_TYPES.Property && !p.computed && p.value === def.name && propName(p.key) === exportName,
      );
    }
    return false;
  }

  // ---------- client proofs ----------

  /** The variable's only definition is `const x = <init>`; returns that init. */
  private simpleInit(v: Variable | null): Expr | undefined {
    const def = Detector.singleDef(v);
    if (!v || !def || def.type !== "Variable") return undefined;
    const decl = def.node;
    if (decl.id !== def.name || !decl.init || !Detector.neverReassigned(v)) return undefined;
    return decl.init;
  }

  private ldClientKind(id: Node, callStart: number): "current" | "legacy" | undefined {
    const v = this.resolve(id);
    if (!v || !this.declaredBefore(v, callStart)) return undefined;
    const init = this.simpleInit(v);
    if (init?.type !== AST_NODE_TYPES.CallExpression || init.optional) return undefined;
    if (this.isSdkExport(init.callee, LD_PACKAGES, "init")) return "current";
    if (this.isSdkExport(init.callee, LD_LEGACY_PACKAGES, "init")) return "legacy";
    return undefined;
  }

  private isOpenFeatureClientVar(v: Variable, callStart: number): boolean {
    const init = this.simpleInit(v);
    if (init?.type !== AST_NODE_TYPES.CallExpression || init.optional) return false;
    const { callee } = init;
    if (callee.type !== AST_NODE_TYPES.MemberExpression || callee.computed || callee.optional) return false;
    if (callee.property.type !== AST_NODE_TYPES.Identifier || callee.property.name !== "getClient") return false;
    if (!this.isSdkExport(callee.object, OPENFEATURE_PACKAGES, "OpenFeature")) return false;
    return this.declaredBefore(v, callStart);
  }

  /**
   * The variable is a `const`/`let` whose whole declarator (initializer
   * included) ends before `pos`, and `pos` is not in a different SwitchCase.
   * That the declaring block encloses `pos` is guaranteed by scope resolution.
   * Known gap (documented, not fixed): a function declared after the const but
   * invoked before the const is initialized is accepted.
   */
  private declaredBefore(v: Variable, pos: number): boolean {
    const def = v.defs[0];
    if (def?.type !== "Variable") return false;
    const declaration = def.parent;
    // `var` hoists (use before the initializer runs sees undefined); only
    // block-scoped declarations have a position-based proof.
    if (declaration?.type !== AST_NODE_TYPES.VariableDeclaration || (declaration.kind !== "const" && declaration.kind !== "let")) {
      return false;
    }
    if (def.node.range[1] > pos) return false;
    // Different SwitchCases share one scope, but another case's declaration never ran (TDZ).
    const clause = this.parents.get(declaration);
    return !(clause?.type === AST_NODE_TYPES.SwitchCase && !(clause.range[0] <= pos && pos < clause.range[1]));
  }

  private bindClient(callStart: number): ClientBinding {
    const callScope = this.innermostScope(callStart);
    for (let s: Scope | null = callScope; s; s = s.upper) {
      const candidates = s.variables.filter((v) => this.isOpenFeatureClientVar(v, callStart));
      if (candidates.length === 0) continue;
      const [only] = candidates;
      if (candidates.length > 1 || !only) return { kind: "ambiguous" };
      // A closer binding of the same name (any kind) shadows the candidate.
      let seen: Variable | undefined;
      for (let t: Scope | null = callScope; t && !seen; t = t.upper) seen = t.set.get(only.name);
      return seen === only ? { kind: "bound", name: only.name } : { kind: "none" };
    }
    return { kind: "none" };
  }

  // ---------- call analysis ----------

  private text(n: Node | undefined): string | undefined {
    // A sequence expression loses its parentheses in ESTree; never copy it.
    if (!n || n.type === AST_NODE_TYPES.SequenceExpression) return undefined;
    return this.source.slice(n.range[0], n.range[1]);
  }

  private call(call: TSESTree.CallExpression): void {
    const { callee } = call;
    if (call.optional || callee.type !== AST_NODE_TYPES.MemberExpression) return;
    if (callee.computed || callee.optional) return;
    if (callee.property.type !== AST_NODE_TYPES.Identifier || callee.object.type !== AST_NODE_TYPES.Identifier) return;
    const match = findRuleByMethod("js", callee.property.name);
    if (!match) return;
    const kind = this.ldClientKind(callee.object, call.range[0]);
    if (!kind) return;
    const { rule } = match;
    // The legacy typings have no typed variation methods: not a recognized call.
    if (kind === "legacy" && rule.skip === undefined) return;
    // The call must match the SDK signature. A spread never does, for any rule.
    // Exact arity is checked only where rules.yaml carries argument positions
    // (rewriting rules); report-only rules have none and are not guessed.
    if (call.arguments.some((a) => a.type === AST_NODE_TYPES.SpreadElement)) return;
    if (rule.skip === undefined) {
      const arity = Math.max(rule.keyArg ?? -1, rule.defaultArg ?? -1, rule.contextArg ?? -1) + 1;
      if (call.arguments.length !== arity) return;
    }
    // A rewriting rule with no JS template is not a JS finding (see
    // validation-parity.md); the planner has nothing to say about it.
    if (rule.skip === undefined && match.language.template === undefined) return;

    const arg = (i: number | undefined): Expr | undefined => {
      if (i === undefined) return undefined;
      for (let j = 0; j <= i; j++) if (call.arguments[j]?.type === AST_NODE_TYPES.SpreadElement) return undefined;
      return call.arguments[i];
    };

    const keyNode = arg(rule.keyArg);
    const key = staticKey(keyNode);
    const defaultNode = arg(rule.defaultArg);

    this.sites.push({
      ruleId: rule.id,
      method: callee.property.name,
      range: [call.range[0], call.range[1]],
      line: call.loc.start.line,
      col: call.loc.start.column + 1,
      keyIsStatic: key !== undefined && this.text(keyNode) !== undefined,
      ...(key !== undefined && { keyValue: key }),
      ...(this.text(keyNode) !== undefined && { keyText: this.text(keyNode)! }),
      ...(this.text(defaultNode) !== undefined && { defaultText: this.text(defaultNode)! }),
      defaultType: defaultType(defaultNode),
      contextShape: this.contextShape(arg(rule.contextArg)),
      clientBinding: this.bindClient(call.range[0]),
    });
  }

  private contextShape(node: Expr | undefined): ContextShape {
    const other: ContextShape = { kind: "other" };
    if (node?.type !== AST_NODE_TYPES.ObjectExpression) return other;
    let keyNode: Node | undefined;
    let sawKind = false;
    for (const p of node.properties) {
      if (p.type !== AST_NODE_TYPES.Property || p.kind !== "init" || p.method || p.computed) return other;
      const name = propName(p.key);
      if (name === "key" && !keyNode) keyNode = p.value;
      else if (name === "kind" && !sawKind && isStringLiteral(p.value) && p.value.value === "user") sawKind = true;
      else return other;
    }
    const keyText = this.text(keyNode);
    return keyText === undefined ? other : { kind: "simple", keyText };
  }
}

function isNode(v: unknown): v is Node {
  return typeof v === "object" && v !== null && typeof (v as { type?: unknown }).type === "string";
}

/** Decoded value of a string literal or an expression-free template literal. */
function staticKey(n: Node | undefined): string | undefined {
  if (!n) return undefined;
  if (isStringLiteral(n)) return n.value;
  if (n.type === AST_NODE_TYPES.TemplateLiteral && n.expressions.length === 0) return n.quasis[0]?.value.cooked ?? undefined;
  return undefined;
}

function isTypeOnlyImport(def: Definition): boolean {
  const spec = def.node as { importKind?: string };
  const decl = def.parent as { importKind?: string } | null;
  return spec.importKind === "type" || decl?.importKind === "type";
}

/** Only JSON-literal leaves: string/number/boolean/null, arrays and plain objects of those. */
function isJsonLiteral(n: Node | null): boolean {
  if (!n) return false;
  switch (n.type) {
    case AST_NODE_TYPES.Literal:
      return !("regex" in n) && !("bigint" in n);
    case AST_NODE_TYPES.TemplateLiteral:
      return n.expressions.length === 0;
    case AST_NODE_TYPES.UnaryExpression:
      return (n.operator === "-" || n.operator === "+") && n.argument.type === AST_NODE_TYPES.Literal && typeof n.argument.value === "number";
    case AST_NODE_TYPES.ArrayExpression:
      return n.elements.every((e) => e !== null && e.type !== AST_NODE_TYPES.SpreadElement && isJsonLiteral(e));
    case AST_NODE_TYPES.ObjectExpression:
      return n.properties.every((p) => {
        if (p.type !== AST_NODE_TYPES.Property || p.kind !== "init" || p.method || p.computed || p.shorthand) return false;
        const name = propName(p.key);
        return name !== undefined && name !== "__proto__" && isJsonLiteral(p.value);
      });
    default:
      return false;
  }
}

function defaultType(n: Node | undefined): DefaultType {
  if (!n) return "unknown";
  switch (n.type) {
    case AST_NODE_TYPES.Literal:
      if (typeof n.value === "boolean") return "boolean";
      if (typeof n.value === "string") return "string";
      if (typeof n.value === "number") return "number";
      return n.value === null && !("regex" in n) && !("bigint" in n) ? "object" : "unknown";
    case AST_NODE_TYPES.TemplateLiteral:
      return n.expressions.length === 0 ? "string" : "unknown";
    case AST_NODE_TYPES.UnaryExpression:
      return (n.operator === "-" || n.operator === "+") && n.argument.type === AST_NODE_TYPES.Literal && typeof n.argument.value === "number"
        ? "number"
        : "unknown";
    case AST_NODE_TYPES.ObjectExpression:
    case AST_NODE_TYPES.ArrayExpression:
      return isJsonLiteral(n) ? "object" : "unknown";
    default:
      return "unknown";
  }
}

/**
 * Find LaunchDarkly evaluation calls whose receiver is proven (through scope
 * references, never by name) to be a client created by the LD server SDK's
 * `init`. `source` is the text `ast` was parsed from
 * (BOM already stripped, as ESLint does) and `visitorKeys` come from the parser.
 */
export function detect(ast: TSESTree.Program, scopes: ScopeManager, source: string, visitorKeys: VisitorKeys): CallSite[] {
  return new Detector(ast, scopes, source, visitorKeys).run();
}
