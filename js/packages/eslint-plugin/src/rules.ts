// SPDX-License-Identifier: MIT
import type { Rule, SourceCode } from "eslint";
import { detect, plan, type CallSite, type ScopeManager } from "@fldlint/core";
import { rules as specRules, skipCategories, type PlanResult, type Rule as SpecRule } from "@fldlint/spec";

interface Finding {
  site: CallSite;
  result: PlanResult;
}

const DOCS_URL = "https://github.com/jh125486/fldlint/tree/main/js/packages/eslint-plugin";

// One detect() per file, shared by every fldlint rule (about 15 of them).
const cache = new WeakMap<object, Finding[]>();

function findings(sourceCode: SourceCode): Finding[] {
  let found = cache.get(sourceCode);
  if (!found) {
    // ESLint strips a leading BOM from sourceCode.text, as core does; ranges agree.
    // The ESTree/scope types differ nominally from typescript-eslint's; the shapes are the same.
    found = detect(
      sourceCode.ast as never,
      sourceCode.scopeManager as unknown as ScopeManager,
      sourceCode.text,
      sourceCode.visitorKeys as never,
    ).map((site) => ({ site, result: plan(site) }));
    cache.set(sourceCode, found);
  }
  return found;
}

const skipMessageId = (category: string): string => `skip-${category}`;

function makeRule(spec: SpecRule): Rule.RuleModule {
  const messages: Record<string, string> = {};
  if (spec.message !== undefined) messages.migrate = spec.message;
  for (const [category, { message }] of Object.entries(skipCategories)) {
    messages[skipMessageId(category)] = `${message} [skip: ${category}]`;
  }
  return {
    meta: {
      type: "problem",
      docs: {
        description: spec.message
          ? `Report LaunchDarkly ${spec.id} calls that can migrate to OpenFeature`
          : `Report LaunchDarkly calls of rule ${spec.id} that need manual migration`,
        url: `${DOCS_URL}#${spec.id}`,
      },
      fixable: "code",
      schema: [],
      messages,
    },
    create(context) {
      return {
        "Program:exit"() {
          const { sourceCode } = context;
          for (const { site, result } of findings(sourceCode)) {
            if (site.ruleId !== spec.id) continue;
            // getLocFromIndex yields 1-based line and 0-based UTF-16 column, which
            // ESLint prints 1-based: the spec's column.
            const loc = {
              start: sourceCode.getLocFromIndex(site.range[0]),
              end: sourceCode.getLocFromIndex(site.range[1]),
            };
            if (result.kind === "rewrite") {
              const { edit } = result;
              context.report({
                loc,
                messageId: "migrate",
                data: { key: site.keyText ?? "" },
                fix: (fixer) => fixer.replaceTextRange(edit.range, edit.text),
              });
            } else {
              context.report({ loc, messageId: skipMessageId(result.category) });
            }
          }
        },
      };
    },
  };
}

/** One ESLint rule per spec rule id that has JS methods; Go-only ids (int-variation, ...) are omitted. */
export function makeRules(): Record<string, Rule.RuleModule> {
  return Object.fromEntries(specRules.filter((r) => r.languages.js).map((r) => [r.id, makeRule(r)]));
}
