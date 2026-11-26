// SPDX-License-Identifier: MIT
// Derived from flaglint-js src/scanner/index.ts @b85599fec7aab9d9af79e018b3e2df3013fc0dd6
// (collectLDClients), MIT, see NOTICE. Receiver proof only: the port keys on
// scope-manager Variable objects instead of identifier names.
import { AST_NODE_TYPES, AST_TOKEN_TYPES, TSESLint, type TSESTree } from "@typescript-eslint/utils";
import { findRuleByMethod } from "@fldlint/spec";
import { visitorKeys as defaultVisitorKeys } from "@typescript-eslint/visitor-keys";
import type { ArgRole, CallComment, CallSite, ClientBinding, CommentNotes, ContextShape, DefaultType } from "./callsite.js";
import { hasLineTerminator, isDirective } from "./expand.js";
import { staticFingerprintKey } from "./fingerprint.js";

const { DefinitionType, ScopeType } = TSESLint.Scope;

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
  private readonly comments: TSESTree.Comment[];

  constructor(
    private readonly ast: TSESTree.Program,
    private readonly scopes: ScopeManager,
    private readonly source: string,
    private readonly visitorKeys: VisitorKeys = defaultVisitorKeys as unknown as VisitorKeys,
  ) {
    for (const scope of scopes.scopes) for (const ref of scope.references) this.refs.set(ref.identifier, ref);
    this.comments = [...(ast.comments ?? [])].sort((a, b) => a.range[0] - b.range[0]);
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
    if (resolved !== null && !(resolved.scope.type === ScopeType.global && resolved.defs.length === 0)) return false;
    // `require = m => ...` makes the global untrustworthy, declared or not.
    if (this.requireIsWritten()) return false;
    const [arg] = expr.arguments;
    return expr.arguments.length === 1 && isStringLiteral(arg) && pkgs.includes(arg.value);
  }

  private requireWritten: boolean | undefined;

  /** Is any global `require` (resolved to the global or unresolved) ever assigned? */
  private requireIsWritten(): boolean {
    this.requireWritten ??= [...this.refs.values()].some(
      (r) => r.identifier.name === "require" && r.isWrite() && (r.resolved === null || r.resolved.scope.type === ScopeType.global),
    );
    return this.requireWritten;
  }

  /** Does `expr` evaluate to the module object of one of `pkgs`? */
  private isModuleObject(expr: Expr, pkgs: readonly string[]): boolean {
    if (this.isRequireOf(expr, pkgs)) return true;
    if (expr.type !== AST_NODE_TYPES.Identifier) return false;
    const v = this.resolve(expr);
    const def = Detector.singleDef(v);
    if (!def || !v) return false;
    if (def.type === DefinitionType.ImportBinding) {
      const spec = def.node;
      if (isTypeOnlyImport(def)) return false;
      const fromPkg = def.parent?.type === AST_NODE_TYPES.ImportDeclaration && pkgs.includes(def.parent.source.value);
      return fromPkg && (spec.type === AST_NODE_TYPES.ImportNamespaceSpecifier || spec.type === AST_NODE_TYPES.ImportDefaultSpecifier);
    }
    if (def.type === DefinitionType.Variable) {
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
    if (def.type === DefinitionType.ImportBinding) {
      const spec = def.node;
      if (spec.type !== AST_NODE_TYPES.ImportSpecifier || isTypeOnlyImport(def)) return false;
      if (def.parent?.type !== AST_NODE_TYPES.ImportDeclaration || !pkgs.includes(def.parent.source.value)) return false;
      return propName(spec.imported) === exportName;
    }
    if (def.type === DefinitionType.Variable) {
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
    if (!v || !def || def.type !== DefinitionType.Variable) return undefined;
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
    if (def?.type !== DefinitionType.Variable) return false;
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
    // Rewriting rules need exactly the arity implied by keyArg/defaultArg/
    // contextArg; report-only rules need an argument count inside their
    // argCount range (rules.yaml).
    if (call.arguments.some((a) => a.type === AST_NODE_TYPES.SpreadElement)) return;
    const count = call.arguments.length;
    if (rule.skip === undefined) {
      const arity = Math.max(rule.keyArg ?? -1, rule.defaultArg ?? -1, rule.contextArg ?? -1) + 1;
      if (count !== arity) return;
    } else {
      const { argCount } = match.language;
      if (argCount && (count < argCount.min || (argCount.max !== undefined && count > argCount.max))) return;
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
    const fpKey = staticFingerprintKey(key);
    const defaultNode = arg(rule.defaultArg);

    const notes = rule.skip === undefined ? this.commentNotes(call, rule) : undefined;
    this.sites.push({
      ruleId: rule.id,
      method: callee.property.name,
      range: [call.range[0], call.range[1]],
      line: call.loc.start.line,
      col: call.loc.start.column + 1,
      keyIsStatic: key !== undefined && this.text(keyNode) !== undefined,
      ...(fpKey !== null && { keyValue: fpKey }),
      ...(this.text(keyNode) !== undefined && { keyText: this.text(keyNode)! }),
      ...(this.text(defaultNode) !== undefined && { defaultText: this.text(defaultNode)! }),
      defaultType: defaultType(defaultNode),
      contextShape: this.contextShape(arg(rule.contextArg)),
      clientBinding: this.bindClient(call.range[0]),
      ...(notes && { comments: notes }),
    });
  }

  /** The `key` value node of a context that is exactly `{ key, kind?: "user" }`. */
  private contextKeyNode(node: Expr | undefined): Node | undefined {
    if (node?.type !== AST_NODE_TYPES.ObjectExpression) return undefined;
    let keyNode: Node | undefined;
    let sawKind = false;
    for (const p of node.properties) {
      if (p.type !== AST_NODE_TYPES.Property || p.kind !== "init" || p.method || p.computed) return undefined;
      const name = propName(p.key);
      if (name === "key" && !keyNode) keyNode = p.value;
      else if (name === "kind" && !sawKind && isStringLiteral(p.value) && p.value.value === "user") sawKind = true;
      else return undefined;
    }
    return keyNode;
  }

  private contextShape(node: Expr | undefined): ContextShape {
    const keyText = this.text(this.contextKeyNode(node));
    return keyText === undefined ? { kind: "other" } : { kind: "simple", keyText };
  }

  // ---------- comments inside a call (issue #31) ----------

  private comment(c: TSESTree.Comment): CallComment {
    const text = this.source.slice(c.range[0], c.range[1]);
    if (c.type !== AST_TOKEN_TYPES.Line) return { text, line: false, eol: "" };
    const rest = this.source.slice(c.range[1], c.range[1] + 2);
    return { text, line: true, eol: rest.startsWith("\r\n") ? "\r\n" : /^[\n\r\u2028\u2029]/.test(rest) ? rest.charAt(0) : "\n" };
  }

  /** Offset of the first `,` at or after `from`, skipping whitespace, comments and closing parens. */
  private commaAfter(from: number, inCall: readonly TSESTree.Comment[]): number {
    let pos = from;
    for (;;) {
      const c = inCall.find((x) => x.range[0] === pos);
      if (c) pos = c.range[1];
      else if (/[\s)]/.test(this.source.charAt(pos))) pos++;
      else return pos;
    }
  }

  /**
   * Place every comment inside the call range: those in an argument's own text
   * are copied with it; the rest attach to the argument they touch, to the
   * head (before the `(`) or to the tail (after the last argument). Returns
   * undefined when the call holds no comment that needs placing.
   */
  private commentNotes(call: TSESTree.CallExpression, rule: { keyArg?: number; defaultArg?: number; contextArg?: number }): CommentNotes | undefined {
    const [start, end] = call.range;
    // `this.comments` is sorted by start: binary-search the first one at or after the call start.
    let lo = 0;
    let hi = this.comments.length;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (this.comments[mid]!.range[0] < start) lo = mid + 1;
      else hi = mid;
    }
    const inCall: TSESTree.Comment[] = [];
    for (let i = lo; i < this.comments.length && this.comments[i]!.range[0] < end; i++) {
      if (this.comments[i]!.range[1] <= end) inCall.push(this.comments[i]!);
    }
    if (inCall.length === 0) return undefined;
    const args = call.arguments;
    const roleOf = (i: number): ArgRole | undefined =>
      i === rule.keyArg ? "key" : i === rule.defaultArg ? "default" : i === rule.contextArg ? "context" : undefined;
    const ctxKey = rule.contextArg === undefined ? undefined : this.contextKeyNode(args[rule.contextArg]);
    const notes: CommentNotes = { args: {}, head: [], tail: [], unhomed: false };
    const slot = (role: ArgRole) => (notes.args[role] ??= { lead: [], trail: [] });

    // The `(`: first non-trivia after the callee (and any type arguments).
    let open = Math.max(call.callee.range[1], call.typeArguments?.range[1] ?? 0);
    for (;;) {
      while (/\s/.test(this.source.charAt(open))) open++;
      const c = inCall.find((x) => x.range[0] === open);
      if (!c) break;
      open = c.range[1];
    }
    if (this.source.charAt(open) !== "(") notes.unhomed = true;

    for (const c of inCall) {
      const [cs, ce] = c.range;
      const inside = args.findIndex((a) => a.range[0] <= cs && ce <= a.range[1]);
      if (inside >= 0) {
        // Copied verbatim with the argument, except inside the converted context literal.
        const role = roleOf(inside);
        const kept = role === "key" || role === "default" || (role === "context" && ctxKey !== undefined && ctxKey.range[0] <= cs && ce <= ctxKey.range[1]);
        if (!kept) notes.unhomed = true;
        continue;
      }
      const cc = this.comment(c);
      // Moving a directive changes what it governs.
      if (isDirective(cc.text)) notes.unhomed = true;
      if (cs < open) {
        notes.head.push(cc);
        continue;
      }
      const next = args.findIndex((a) => a.range[0] >= ce);
      const prevIdx = (next < 0 ? args.length : next) - 1;
      const prev = args[prevIdx];
      const following = next < 0 ? undefined : args[next];
      const attach = (i: number, where: "lead" | "trail"): void => {
        const role = roleOf(i);
        if (role === undefined) notes.unhomed = true;
        else slot(role)[where].push(cc);
      };
      if (!prev) attach(next, "lead");
      else if (!following) {
        if (hasLineTerminator(this.source.slice(prev.range[1], cs))) notes.tail.push(cc);
        else attach(prevIdx, "trail");
      } else if (cs < this.commaAfter(prev.range[1], inCall)) {
        // Before the separating comma: it touches the previous argument.
        attach(prevIdx, "trail");
      } else if (!hasLineTerminator(this.source.slice(prev.range[1], cs)) && (c.type === AST_TOKEN_TYPES.Line || hasLineTerminator(this.source.slice(ce, following.range[0])))) {
        attach(prevIdx, "trail");
      } else attach(next, "lead");
    }
    return notes.head.length === 0 && notes.tail.length === 0 && Object.keys(notes.args).length === 0 && !notes.unhomed ? undefined : notes;
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
 * (BOM already stripped, as ESLint does). `visitorKeys` should be the parser's
 * (`sourceCode.visitorKeys` in ESLint); when omitted it falls back to
 * `@typescript-eslint/visitor-keys`.
 */
export function detect(ast: TSESTree.Program, scopes: ScopeManager, source: string, visitorKeys?: VisitorKeys): CallSite[] {
  return new Detector(ast, scopes, source, visitorKeys).run();
}
