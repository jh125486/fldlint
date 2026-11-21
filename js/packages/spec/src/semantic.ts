import { existsSync } from "node:fs";
import { basename, dirname, join } from "node:path";
import type { CaseFile } from "./load.js";
import { LANGUAGES, type Language, type RulesFile } from "./types.js";

/** Per-language fixture file names inside a case directory. */
export const LANG_FILES: Readonly<Record<Language, { input: string; output: string }>> = {
  js: { input: "js/input.ts", output: "js/output.ts" },
  go: { input: "go/input.go", output: "go/output.go.golden" },
  java: { input: "java/Before.java", output: "java/After.java" },
};

/** Cross-reference checks on rules.yaml that JSON Schema cannot express. */
export function validateRulesSemantics(rf: RulesFile): string[] {
  const errors: string[] = [];
  const ids = new Set<string>();
  for (const r of rf.rules) {
    if (ids.has(r.id)) errors.push(`rules.yaml: duplicate rule id "${r.id}"`);
    ids.add(r.id);
    for (const lang of LANGUAGES) {
      const cfg = r.languages[lang];
      if (!cfg || !cfg.ctxArg) continue;
      for (const m of Object.keys(cfg.ctxArg)) {
        if (!cfg.methods.includes(m)) {
          errors.push(`rules.yaml: rule "${r.id}" ${lang}.ctxArg key "${m}" is not in ${lang}.methods`);
        }
      }
    }
  }
  errors.push(...findDuplicateMethods(rf));
  return errors;
}

/** Report each `lang:method` that appears in more than one rule (or twice within one rule). */
export function findDuplicateMethods(rf: RulesFile): string[] {
  const seen = new Map<string, string>();
  const errors: string[] = [];
  for (const r of rf.rules) {
    for (const lang of LANGUAGES) {
      for (const m of r.languages[lang]?.methods ?? []) {
        const key = `${lang}:${m}`;
        const prev = seen.get(key);
        if (prev !== undefined) {
          errors.push(`rules.yaml: ${lang} method "${m}" appears in rules "${prev}" and "${r.id}"`);
        } else seen.set(key, r.id);
      }
    }
  }
  return errors;
}

interface CaseShape {
  rule: string;
  expect: Partial<Record<Language, { diagnostics: { ruleId: string }[]; output: boolean }>>;
}

/**
 * Semantic checks for a case that already passed schema validation:
 * known rule ids, directory/rule agreement, and fixture-file presence.
 */
export function validateCaseSemantics(rf: RulesFile, c: CaseFile): string[] {
  const errors: string[] = [];
  if (!c.data || typeof c.data !== "object" || !("expect" in c.data)) {
    return [`${c.relPath}: not a schema-valid case; semantic checks skipped`];
  }
  const d = c.data as CaseShape;
  const known = new Set(rf.rules.map((r) => r.id));
  const err = (msg: string): void => {
    errors.push(`${c.relPath}: ${msg}`);
  };
  if (!known.has(d.rule)) err(`rule "${d.rule}" is not defined in rules.yaml`);
  const dirRule = basename(dirname(c.dir));
  if (dirRule !== d.rule) err(`rule "${d.rule}" does not match its directory name "${dirRule}"`);
  for (const lang of LANGUAGES) {
    const files = LANG_FILES[lang];
    const exp = d.expect[lang];
    const hasInput = existsSync(join(c.dir, files.input));
    const hasOutput = existsSync(join(c.dir, files.output));
    if (!exp) {
      if (hasOutput) err(`${files.output} exists but case.yaml has no expect.${lang}`);
      continue;
    }
    if (!hasInput) err(`expect.${lang} requires ${files.input}, which does not exist`);
    if (exp.output && !hasOutput) err(`expect.${lang}.output is true but ${files.output} does not exist`);
    if (!exp.output && hasOutput) err(`expect.${lang}.output is false but ${files.output} exists`);
    for (const diag of exp.diagnostics) {
      if (!known.has(diag.ruleId)) {
        err(`expect.${lang}.diagnostics ruleId "${diag.ruleId}" is not defined in rules.yaml`);
      }
    }
  }
  return errors;
}
