import { findRuleByMethod } from "./lookup.js";
import type { Language } from "./types.js";

export interface Edit {
  /** Half-open [start, end) byte offsets in the source file. */
  range: [number, number];
  text: string;
}

export type PlanResult =
  | { kind: "rewrite"; edit: Edit }
  | { kind: "skip"; category: string; ruleId: string };

/** A detected call site, as handed over by a language's detection engine. */
export interface CallSite {
  language: Language;
  method: string;
}

/**
 * Stub planner (M0). Real detection arrives in M1; until then every call site
 * is reported as skipped because client binding cannot be proven.
 */
export function plan(site: CallSite): PlanResult {
  const match = findRuleByMethod(site.language, site.method);
  return { kind: "skip", category: "unbound-client", ruleId: match?.rule.id ?? "" };
}
