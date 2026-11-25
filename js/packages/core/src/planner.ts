import { rules, type Rule } from "@fldlint/spec";
import type { CallSite, DefaultType } from "./callsite.js";
import type { PlanResult } from "@fldlint/spec";
import { expand } from "./expand.js";

const rulesById = new Map<string, Rule>(rules.map((r) => [r.id, r]));

function defaultMatches(valueType: Rule["valueType"], actual: DefaultType): boolean {
  if (actual === "unknown") return false;
  // `object` rules take any JSON-shaped literal.
  return valueType === "object" || valueType === actual;
}

/**
 * Plan one call site. Pure and AST-agnostic: everything it needs is in `site`.
 *
 * Step order (one category per call, first match wins):
 * rule-level skip, dynamic-key, context-unproven, default-type-unproven,
 * unbound-client, else rewrite (or, when a comment inside the call has no home
 * in the rewritten call, report it without a fix). (A call whose receiver is not proven is never
 * a call site, so it yields no finding at all.)
 */
export function plan(site: CallSite): PlanResult {
  const rule = rulesById.get(site.ruleId);
  if (!rule) throw new Error(`unknown rule id: ${site.ruleId}`);
  const skip = (category: string): PlanResult => ({ kind: "skip", category, ruleId: rule.id });

  if (rule.skip) return skip(rule.skip);
  const template = rule.languages.js?.template;
  if (template === undefined) throw new Error(`rule ${rule.id} has no js template`);

  if (!site.keyIsStatic || site.keyText === undefined) return skip("dynamic-key");
  if (site.contextShape.kind !== "simple") return skip("context-unproven");
  if (site.defaultText === undefined || !defaultMatches(rule.valueType, site.defaultType)) {
    return skip("default-type-unproven");
  }
  if (site.clientBinding.kind !== "bound") return skip("unbound-client");

  const values: Record<string, string> = {
    client: site.clientBinding.name,
    key: site.keyText,
    default: site.defaultText,
    context: `{ targetingKey: ${site.contextShape.keyText} }`,
  };
  // Comments inside the call move with their arguments; one with no home
  // withholds the fix (the call is still reported). Not a skip category.
  if (site.comments?.unhomed) return { kind: "unfixable", ruleId: rule.id };
  const text = expand(template, values, site.comments);
  if (text === undefined) return { kind: "unfixable", ruleId: rule.id };
  return { kind: "rewrite", edit: { range: site.range, text } };
}
