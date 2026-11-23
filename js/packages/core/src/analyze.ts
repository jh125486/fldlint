import type { Edit, PlanResult } from "@fldlint/spec";
import type { CallSite } from "./callsite.js";
import { detect } from "./detect.js";
import { applyEdits, nonOverlapping } from "./edits.js";
import { parseSource } from "./parse.js";
import { plan } from "./planner.js";

export interface Finding {
  site: CallSite;
  result: PlanResult;
}

/** Parse, detect and plan in one step. */
export function analyze(source: string, filePath?: string): Finding[] {
  const { ast, scopeManager, visitorKeys, source: text } = parseSource(source, filePath);
  // Ranges are reported in the coordinates of `source` as given (BOM included),
  // so they can be handed straight to applyEdits(source, ...). line/col follow
  // ESLint, which never counts the BOM.
  const shift = source.length - text.length;
  return detect(ast, scopeManager, text, visitorKeys).map((raw) => {
    const site: CallSite = { ...raw, range: [raw.range[0] + shift, raw.range[1] + shift] };
    return { site, result: plan(site) };
  });
}

/**
 * Apply every planned rewrite in one pass. Nested rewrites (an outer call whose
 * argument text contains another rewritable call) keep only the outer edit;
 * run again to pick up the rest.
 */
export function rewrite(source: string, filePath?: string): string {
  const edits: Edit[] = [];
  for (const f of analyze(source, filePath)) if (f.result.kind === "rewrite") edits.push(f.result.edit);
  return applyEdits(source, nonOverlapping(edits));
}
