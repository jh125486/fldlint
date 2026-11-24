import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { discoverCases, loadRules } from "./load.js";
import { LANG_FILES } from "./semantic.js";
import { type Language } from "./types.js";

/** The reference rule that must cover every shape tag. */
export const REFERENCE_RULE = "bool-variation";
/** Rewrite coverage is required for js and go only (M0.2b); java arrives in M3. */
export const COVERAGE_LANGUAGES: readonly Language[] = ["js", "go"];
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
  expect?: Record<string, { output?: unknown; diagnostics?: unknown } | undefined>;
}

interface LoadedCase {
  data: LooseCase;
  dir: string;
}

/** A real rewrite case: output true, >=1 diagnostic, output file present and different from input. */
function isRewriteFor(c: LoadedCase, lang: Language): boolean {
  const e = c.data.expect?.[lang];
  if (!e || e.output !== true) return false;
  if (!Array.isArray(e.diagnostics) || e.diagnostics.length === 0) return false;
  if (e.diagnostics.some((d) => (d as { skip?: unknown })?.skip !== undefined)) return false;
  try {
    const input = readFileSync(join(c.dir, LANG_FILES[lang].input));
    const output = readFileSync(join(c.dir, LANG_FILES[lang].output));
    return !input.equals(output);
  } catch {
    return false;
  }
}

/**
 * Fixture coverage: the reference rule needs the seven mandatory tags; every
 * rewriting rule (no `skip`) needs a rewrite case (output: true, >=1
 * diagnostic, output differing from input) for each language with a template.
 */
export function checkTags(specRoot: string, opts: { allowEmpty?: boolean } = {}): TagsResult {
  if (!existsSync(join(specRoot, "fixtures"))) {
    if (opts.allowEmpty) return { errors: [], note: `no fixtures directory under ${specRoot} (allowed by --allow-empty)` };
    return { errors: [`fixtures directory not found under ${specRoot}`] };
  }
  let rf;
  try {
    rf = loadRules(specRoot);
  } catch (e) {
    return { errors: [`rules.yaml: ${(e as Error).message}`] };
  }
  if (!rf || !Array.isArray(rf.rules)) return { errors: ["rules.yaml: no rules list"] };
  const cases: LoadedCase[] = discoverCases(specRoot).flatMap((c) =>
    c.data && typeof c.data === "object" ? [{ data: c.data, dir: c.dir }] : [],
  );
  const errors: string[] = [];
  if (!rf.rules.some((r) => r.id === REFERENCE_RULE)) {
    errors.push(`reference rule "${REFERENCE_RULE}" is not defined in rules.yaml`);
  }
  for (const rule of rf.rules) {
    if (rule.skip !== undefined) continue;
    const mine = cases.filter((c) => c.data.rule === rule.id);
    if (rule.id === REFERENCE_RULE) {
      const have = new Set(mine.flatMap((c) => (Array.isArray(c.data.tags) ? (c.data.tags as unknown[]) : [])));
      for (const tag of MANDATORY_TAGS) {
        if (!have.has(tag)) errors.push(`rule "${rule.id}": missing mandatory tag "${tag}"`);
      }
    }
    for (const lang of COVERAGE_LANGUAGES) {
      if (rule.languages[lang]?.template === undefined) continue;
      if (!mine.some((c) => isRewriteFor(c, lang))) {
        errors.push(
          `rule "${rule.id}": no rewrite case for language "${lang}" (expect.${lang}.output: true with >=1 diagnostic and an output file differing from the input)`,
        );
      }
    }
  }
  return { errors };
}
