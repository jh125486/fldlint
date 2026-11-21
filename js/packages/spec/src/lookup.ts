import { rules, skipCategories } from "./rules.gen.js";
import { findDuplicateMethods } from "./semantic.js";
import type { Language, LanguageRule, Rule, RulesFile } from "./types.js";

export interface RuleMatch {
  rule: Rule;
  language: LanguageRule;
}

/** Build the method index; throws if a method maps to more than one rule. */
export function buildMethodIndex(rs: readonly Rule[]): Map<string, RuleMatch> {
  const dups = findDuplicateMethods({ version: 0, skipCategories: {}, rules: [...rs] } satisfies RulesFile);
  if (dups.length > 0) throw new Error(`duplicate rule methods: ${dups.join("; ")}`);
  const m = new Map<string, RuleMatch>();
  for (const rule of rs) {
    for (const [lang, cfg] of Object.entries(rule.languages)) {
      for (const method of cfg.methods) m.set(`${lang}:${method}`, { rule, language: cfg });
    }
  }
  return m;
}

const index = buildMethodIndex(rules);
/** Look up a rule by language method name (e.g. `js`, `boolVariation`). */
export function findRuleByMethod(language: Language, method: string): RuleMatch | undefined {
  return index.get(`${language}:${method}`);
}

export function isSkipCategory(name: string): boolean {
  return Object.hasOwn(skipCategories, name);
}
