// Metamorphic tests (issue #21, part 1): over every fixture, a change that does
// not touch the calls must not change what is found. The multiset of
// (ruleId, skip, key) and the fingerprints stay equal under
//   - blank lines and comments added elsewhere in the file (fingerprint.md,
//     "Stability");
//   - a consistent rename of the LD client variable;
//   - a switch of string quote style.
// Positions may change and are not compared.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { AST_NODE_TYPES, type TSESTree } from "@typescript-eslint/utils";
import { describe, expect, it } from "vitest";
import { discoverCases, resolveSpecRoot, rules } from "@fldlint/spec";
import { analyze, assignOccurrences, fingerprint, parseSource } from "../src/index.js";

const root = resolveSpecRoot();
const PATH = "input.ts";
const BOM = "﻿";

interface Identity {
  ruleId: string;
  skip: string;
  key: string | null;
  line: number;
  col: number;
}

function identities(source: string): Identity[] {
  return analyze(source, PATH).map(({ site, result }) => ({
    ruleId: site.ruleId,
    skip: result.kind === "skip" ? result.category : result.kind === "unfixable" ? "no-fix" : "",
    key: site.keyValue ?? null,
    line: site.line,
    col: site.col,
  }));
}

/** Sorted (ruleId, skip, key) multiset and sorted fingerprints of a source. */
function summary(source: string): { multiset: string[]; fingerprints: string[] } {
  const ids = identities(source);
  const withOcc = assignOccurrences(ids.map((i) => ({ ...i })));
  return {
    multiset: ids.map((i) => JSON.stringify([i.ruleId, i.skip, i.key])).sort(),
    fingerprints: withOcc.map((f) => fingerprint({ ruleId: f.ruleId, path: PATH, key: f.key, occ: f.occ })).sort(),
  };
}

interface Edit {
  start: number;
  end: number;
  text: string;
}

function apply(source: string, edits: Edit[]): string {
  let out = source;
  for (const e of [...edits].sort((a, b) => b.start - a.start)) out = out.slice(0, e.start) + e.text + out.slice(e.end);
  return out;
}

type Node = TSESTree.Node;

/** Pre-order walk with parent links; `keys` are the parser's visitor keys. */
function walk(node: Node, keys: Record<string, readonly string[] | undefined>, visit: (n: Node, parent: Node | null) => void, parent: Node | null = null): void {
  visit(node, parent);
  for (const k of keys[node.type] ?? []) {
    const child = (node as unknown as Record<string, unknown>)[k];
    for (const c of Array.isArray(child) ? child : [child]) {
      if (c !== null && typeof c === "object" && "type" in c) walk(c as Node, keys, visit, node);
    }
  }
}

type Transform = (source: string) => string | null;

/** Blank lines and comments before every top-level statement, and at the end. */
const withNoise: Transform = (source) => {
  const { ast } = parseSource(source, PATH);
  const floor = source.startsWith(BOM) ? 1 : 0;
  const edits: Edit[] = [];
  const seen = new Set<number>();
  for (const st of ast.body) {
    const start = st.range[0] + (source.startsWith(BOM) ? 1 : 0);
    const lineStart = Math.max(floor, source.lastIndexOf("\n", start - 1) + 1);
    const atLineStart = source.slice(lineStart, start).trim() === "";
    const at = atLineStart ? lineStart : start;
    if (seen.has(at)) continue;
    seen.add(at);
    edits.push({ start: at, end: at, text: atLineStart ? "// metamorphic noise\n\n\n/* more noise */\n\n" : "/* noise */ " });
  }
  return apply(source, edits) + "\n\n// trailing noise\n";
};

/** Rename every variable that an SDK method is called on. */
const withRenamedClient: Transform = (source) => {
  const { ast, scopeManager, visitorKeys } = parseSource(source, PATH);
  const shift = source.startsWith(BOM) ? 1 : 0;
  const methods = new Set(rules.flatMap((r) => r.languages.js?.methods ?? []));
  const refs = new Map<Node, { resolved: unknown }>();
  for (const scope of scopeManager.scopes) for (const ref of scope.references) refs.set(ref.identifier, ref);
  const parents = new Map<Node, Node | null>();
  const receivers: TSESTree.Identifier[] = [];
  walk(ast, visitorKeys, (n, parent) => {
    parents.set(n, parent);
    if (n.type !== AST_NODE_TYPES.CallExpression || n.callee.type !== AST_NODE_TYPES.MemberExpression || n.callee.computed) return;
    if (n.callee.property.type !== AST_NODE_TYPES.Identifier || !methods.has(n.callee.property.name)) return;
    if (n.callee.object.type === AST_NODE_TYPES.Identifier) receivers.push(n.callee.object);
  });
  const edits: Edit[] = [];
  const done = new Set<unknown>();
  let n = 0;
  for (const id of receivers) {
    const variable = refs.get(id)?.resolved as { identifiers: Node[]; references: { identifier: Node }[] } | null | undefined;
    if (!variable || done.has(variable) || variable.identifiers.length === 0) continue;
    const ids = [...variable.identifiers, ...variable.references.map((r) => r.identifier)];
    // A shorthand property, an export or an import specifier would change meaning.
    if (ids.some((x) => {
      const p = parents.get(x);
      return (p?.type === AST_NODE_TYPES.Property && p.shorthand) || p?.type === AST_NODE_TYPES.ExportSpecifier || p?.type === AST_NODE_TYPES.ImportSpecifier;
    })) continue;
    done.add(variable);
    const name = `ldRenamed${n++}`;
    for (const x of new Set(ids)) edits.push({ start: x.range[0] + shift, end: x.range[1] + shift, text: name });
  }
  return edits.length === 0 ? null : apply(source, edits);
};

/** Switch every simple string literal between single and double quotes. */
const withSwitchedQuotes: Transform = (source) => {
  const { ast, visitorKeys } = parseSource(source, PATH);
  const shift = source.startsWith(BOM) ? 1 : 0;
  const edits: Edit[] = [];
  walk(ast, visitorKeys, (n) => {
    if (n.type !== AST_NODE_TYPES.Literal || typeof n.value !== "string") return;
    const q = n.raw[0];
    const inner = n.raw.slice(1, -1);
    if ((q !== '"' && q !== "'") || /['"\\\n\r]/.test(inner)) return;
    const to = q === '"' ? "'" : '"';
    edits.push({ start: n.range[0] + shift, end: n.range[1] + shift, text: `${to}${inner}${to}` });
  });
  return edits.length === 0 ? null : apply(source, edits);
};

const transforms: { name: string; fn: Transform }[] = [
  { name: "blank lines and comments elsewhere", fn: withNoise },
  { name: "renamed LD client variable", fn: withRenamedClient },
  { name: "quote style switched", fn: withSwitchedQuotes },
];

describe("metamorphic: findings and fingerprints are stable", () => {
  for (const c of discoverCases(root)) {
    const expected = (c.data as { expect?: { js?: unknown } } | undefined)?.expect?.js;
    if (!expected) continue;
    const input = readFileSync(join(c.dir, "js", "input.ts"), "utf8");
    describe(c.relPath, () => {
      const want = summary(input);
      for (const t of transforms) {
        it(t.name, () => {
          const changed = t.fn(input);
          if (changed === null) return; // the transformation does not apply to this fixture
          expect(changed).not.toBe(input);
          const got = summary(changed);
          expect(got.multiset, `(ruleId, skip, key) multiset changed\n${changed}`).toEqual(want.multiset);
          expect(got.fingerprints, "fingerprints changed").toEqual(want.fingerprints);
        });
      }
    });
  }

  it("applies each transformation to at least one fixture", () => {
    const applied = new Map(transforms.map((t) => [t.name, 0]));
    for (const c of discoverCases(root)) {
      if (!(c.data as { expect?: { js?: unknown } } | undefined)?.expect?.js) continue;
      const input = readFileSync(join(c.dir, "js", "input.ts"), "utf8");
      for (const t of transforms) if (t.fn(input) !== null) applied.set(t.name, (applied.get(t.name) ?? 0) + 1);
    }
    for (const [name, n] of applied) expect(n, name).toBeGreaterThan(0);
  });
});
