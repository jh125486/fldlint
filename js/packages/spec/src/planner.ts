import { findRuleByMethod } from "./lookup.js";
import type { Language } from "./types.js";

export interface Edit {
  /** Half-open [start, end) UTF-16 code unit indices in the source file. */
  range: [number, number];
  text: string;
}

export type PlanResult =
  | { kind: "rewrite"; edit: Edit }
  /**
   * A rewritable call whose fix is withheld because a comment inside it has no
   * home in the rewritten call (issue #31). Not a skip category: the finding
   * keeps the rule's own message and carries no fix.
   */
  | { kind: "unfixable"; ruleId: string }
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
