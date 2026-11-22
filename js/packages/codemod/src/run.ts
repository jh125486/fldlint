import { skipCategories } from "@fldlint/spec";
import { analyze, applyEdits, nonOverlapping } from "@fldlint/core";
import type { Edit } from "@fldlint/spec";

/** Safety cap on passes; nesting depth N needs N passes, so this is far beyond real code. */
const MAX_PASSES = 100;
const BOM = "﻿";

export interface RunResult {
  /** New source, or `null` when nothing was rewritten (file stays untouched). */
  output: string | null;
  /** Skip and parse-failure lines, in the shared text shape, in source order. */
  reports: string[];
}

function rewritePass(source: string, filePath: string) {
  const findings = analyze(source, filePath);
  const edits: Edit[] = [];
  for (const f of findings) if (f.result.kind === "rewrite") edits.push(f.result.edit);
  if (edits.length === 0) return { findings, next: source };
  // Ranges index into the BOM-stripped text; keep the BOM on the way out.
  const bom = source.startsWith(BOM);
  const out = applyEdits(bom ? source.slice(1) : source, nonOverlapping(edits));
  return { findings, next: bom ? BOM + out : out };
}

/**
 * The whole codemod, as a pure function: every rewrite and every skip decision
 * comes from `@fldlint/core`; nothing is re-implemented here. Nested rewrites
 * need more than one pass, so it runs to a fixed point (like `eslint --fix`).
 * A file that does not parse, or that has not settled after the pass cap, is
 * left untouched and reported.
 */
export function runCodemod(source: string, filePath: string, maxPasses = MAX_PASSES): RunResult {
  let current = source;
  const reports: string[] = [];
  try {
    let settled = false;
    for (let pass = 0; pass < maxPasses; pass++) {
      const { findings, next } = rewritePass(current, filePath);
      if (pass === 0) {
        // Positions refer to the original file, so only the first pass reports.
        for (const { site, result } of findings) {
          if (result.kind !== "skip") continue;
          const message = skipCategories[result.category]?.message ?? result.category;
          reports.push(`${filePath}:${site.line}:${site.col}  ${result.ruleId}  ${message} [skip: ${result.category}]`);
        }
      }
      if (next === current) {
        settled = true;
        break;
      }
      current = next;
    }
    if (!settled) {
      // Never hand back a partial rewrite: keep the original text.
      return {
        output: null,
        reports: [`${filePath}  max-passes  rewrites still pending after ${maxPasses} passes, left unchanged`],
      };
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message.split("\n")[0] : String(err);
    return { output: null, reports: [`${filePath}  parse-error  could not analyze file, left unchanged: ${msg}`] };
  }
  return { output: current === source ? null : current, reports };
}
