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

const LD_PACKAGES: readonly string[] = ["@launchdarkly/node-server-sdk", "launchdarkly-node-server-sdk"];
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

  constructor(
    private readonly ast: TSESTree.Program,
    private readonly scopes: ScopeManager,
    private readonly source: string,
  ) {
    for (const scope of scopes.scopes) for (const ref of scope.references) this.refs.set(ref.identifier, ref);
  }

  run(): CallSite[] {
    this.walk(this.ast);
    return this.sites;
  }

  private walk(node: Node): void {
    if (node.type === AST_NODE_TYPES.CallExpression) this.call(node);
    for (const [key, value] of Object.entries(node)) {
      if (key === "parent" || key === "loc" || key === "range") continue;
      if (Array.isArray(value)) {
        for (const v of value) if (isNode(v)) this.walk(v);
      } else if (isNode(value)) {
        this.walk(value);
      }
    }
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
    if (this.resolve(callee) !== null) return false;
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
      if (spec.type !== AST_NODE_TYPES.ImportSpecifier) return false;
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

  private isLdClient(id: Node): boolean {
    const init = this.simpleInit(this.resolve(id));
    return (
      init?.type === AST_NODE_TYPES.CallExpression && !init.optional && this.isSdkExport(init.callee, LD_PACKAGES, "init")
    );
  }

  private isOpenFeatureClientVar(v: Variable, callStart: number): boolean {
    const init = this.simpleInit(v);
    if (init?.type !== AST_NODE_TYPES.CallExpression || init.optional) return false;
    const { callee } = init;
    if (callee.type !== AST_NODE_TYPES.MemberExpression || callee.computed || callee.optional) return false;
    if (callee.property.type !== AST_NODE_TYPES.Identifier || callee.property.name !== "getClient") return false;
    if (!this.isSdkExport(callee.object, OPENFEATURE_PACKAGES, "OpenFeature")) return false;
    const def = v.defs[0]!;
    return def.name.range[1] <= callStart; // declared before use
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
    if (!match || !this.isLdClient(callee.object)) return;
    const { rule } = match;

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
      return "object";
    default:
      return "unknown";
  }
}

/**
 * Find LaunchDarkly evaluation calls whose receiver is proven (through scope
 * references, never by name) to be a client created by the LD server SDK's
 * `init`. `source` is the text `ast` was parsed from.
 */
export function detect(ast: TSESTree.Program, scopes: ScopeManager, source: string): CallSite[] {
  return new Detector(ast, scopes, source).run();
}
