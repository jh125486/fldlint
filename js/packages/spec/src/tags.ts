import { existsSync } from "node:fs";
import { join } from "node:path";
import { discoverCases, loadRules } from "./load.js";

/** The reference rule that must cover every shape tag. */
export const REFERENCE_RULE = "bool-variation";
export const MANDATORY_TAGS: readonly string[] = [
  "quote-single",
  "quote-double",
  "typed-decl",
  "multi-declarator",
  "shadowed-client",
  "dynamic-key",
  "aliased-import",
];

export interface TagsResult {
  errors: string[];
  /** Set when the check was skipped (allowEmpty and no fixtures dir). */
  note?: string;
}

interface LooseCase {
  rule?: unknown;
  tags?: unknown;
  expect?: Record<string, { output?: unknown } | undefined>;
}

/**
 * Fixture coverage: the reference rule needs the seven mandatory tags; every
 * other rewriting rule (no `skip`) needs at least one case with `output: true`.
 */
export function checkTags(specRoot: string, opts: { allowEmpty?: boolean } = {}): TagsResult {
  if (!existsSync(join(specRoot, "fixtures"))) {
    if (opts.allowEmpty) return { errors: [], note: `no fixtures directory under ${specRoot} (allowed by --allow-empty)` };
    return { errors: [`fixtures directory not found under ${specRoot}`] };
  }
  const rf = loadRules(specRoot);
  const cases = discoverCases(specRoot).flatMap((c) =>
    c.data && typeof c.data === "object" ? [c.data as LooseCase] : [],
  );
  const errors: string[] = [];
  if (!rf.rules.some((r) => r.id === REFERENCE_RULE)) {
    errors.push(`reference rule "${REFERENCE_RULE}" is not defined in rules.yaml`);
  }
  for (const rule of rf.rules) {
    if (rule.skip !== undefined) continue;
    const mine = cases.filter((c) => c.rule === rule.id);
    if (rule.id === REFERENCE_RULE) {
      const have = new Set(mine.flatMap((c) => (Array.isArray(c.tags) ? (c.tags as unknown[]) : [])));
      for (const tag of MANDATORY_TAGS) {
        if (!have.has(tag)) errors.push(`rule "${rule.id}": missing mandatory tag "${tag}"`);
      }
    }
    const rewrites = mine.some((c) => Object.values(c.expect ?? {}).some((e) => e?.output === true));
    if (!rewrites) errors.push(`rule "${rule.id}": no case with a rewrite (expect.<lang>.output: true)`);
  }
  return { errors };
}
