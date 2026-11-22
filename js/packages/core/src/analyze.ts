import type { Edit, PlanResult } from "@fldlint/spec";
import type { CallSite } from "./callsite.js";
import { detect } from "./detect.js";
import { applyEdits, nonOverlapping } from "./edits.js";
import { BOM, parseSource } from "./parse.js";
import { plan } from "./planner.js";

export interface Finding {
  site: CallSite;
  result: PlanResult;
}

/** Parse, detect and plan in one step. */
export function analyze(source: string, filePath?: string): Finding[] {
  const { ast, scopeManager, visitorKeys, source: text } = parseSource(source, filePath);
  return detect(ast, scopeManager, text, visitorKeys).map((site) => ({ site, result: plan(site) }));
}

/**
 * Apply every planned rewrite in one pass. Nested rewrites (an outer call whose
 * argument text contains another rewritable call) keep only the outer edit;
 * run again to pick up the rest.
 */
export function rewrite(source: string, filePath?: string): string {
  const edits: Edit[] = [];
  for (const f of analyze(source, filePath)) if (f.result.kind === "rewrite") edits.push(f.result.edit);
  // Site ranges index into the BOM-stripped text; keep the BOM on the way out.
  const bom = source.startsWith(BOM);
  const out = applyEdits(bom ? source.slice(1) : source, nonOverlapping(edits));
  return bom ? BOM + out : out;
}
