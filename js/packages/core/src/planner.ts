import { rules, type Rule } from "@fldlint/spec";
import type { CallSite, DefaultType } from "./callsite.js";
import type { PlanResult } from "@fldlint/spec";

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
 * unbound-client, else rewrite. (A call whose receiver is not proven is never
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
  // Single pass so substituted source text is never re-expanded.
  const text = template.replace(/\{\{(\w+)\}\}/g, (m, name: string) => values[name] ?? m);
  return { kind: "rewrite", edit: { range: site.range, text } };
}
