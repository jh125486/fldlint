import { rules, skipCategories } from "./rules.gen.js";
import type { Language, LanguageRule, Rule } from "./types.js";

export interface RuleMatch {
  rule: Rule;
  language: LanguageRule;
}

const index = new Map<string, RuleMatch>();
for (const rule of rules) {
  for (const [lang, cfg] of Object.entries(rule.languages)) {
    for (const m of cfg.methods) index.set(`${lang}:${m}`, { rule, language: cfg });
  }
}

/** Look up a rule by language method name (e.g. `js`, `boolVariation`). */
export function findRuleByMethod(language: Language, method: string): RuleMatch | undefined {
  return index.get(`${language}:${method}`);
}

export function isSkipCategory(name: string): boolean {
  return Object.hasOwn(skipCategories, name);
}
