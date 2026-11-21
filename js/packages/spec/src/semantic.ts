import { existsSync, readFileSync } from "node:fs";
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

interface Diag {
  ruleId: string;
  skip?: string;
}
interface CaseShape {
  id?: string;
  rule: string;
  expect: Partial<Record<Language, { diagnostics: Diag[]; output: boolean }>>;
}

function sameBytes(a: string, b: string): boolean {
  return readFileSync(a).equals(readFileSync(b));
}

/**
 * Semantic checks for a case that already passed schema validation:
 * rule agreement, diagnostic/skip consistency, language binding and
 * fixture-file presence.
 */
export function validateCaseSemantics(rf: RulesFile, c: CaseFile): string[] {
  const errors: string[] = [];
  if (!c.data || typeof c.data !== "object" || !("expect" in c.data)) {
    return [`${c.relPath}: not a schema-valid case; semantic checks skipped`];
  }
  const d = c.data as CaseShape;
  const rule = rf.rules.find((r) => r.id === d.rule);
  const err = (msg: string): void => {
    errors.push(`${c.relPath}: ${msg}`);
  };
  if (!rule) err(`rule "${d.rule}" is not defined in rules.yaml`);
  const dirRule = basename(dirname(c.dir));
  if (dirRule !== d.rule) err(`rule "${d.rule}" does not match its directory name "${dirRule}"`);
  for (const lang of LANGUAGES) {
    const files = LANG_FILES[lang];
    const exp = d.expect[lang];
    const inputPath = join(c.dir, files.input);
    const outputPath = join(c.dir, files.output);
    const hasInput = existsSync(inputPath);
    const hasOutput = existsSync(outputPath);
    if (!exp) {
      if (hasOutput) err(`${files.output} exists but case.yaml has no expect.${lang}`);
      if (hasInput) err(`${files.input} exists but case.yaml has no expect.${lang}`);
      continue;
    }
    const cfg = rule?.languages[lang];
    if (rule && !cfg) err(`expect.${lang} is not allowed: rule "${rule.id}" does not bind language ${lang}`);
    if (rule && cfg && rule.skip === undefined && cfg.template === undefined) {
      err(`expect.${lang} is not allowed: rewriting rule "${rule.id}" has no ${lang} template`);
    }
    if (!hasInput) err(`expect.${lang} requires ${files.input}, which does not exist`);
    if (exp.output && !hasOutput) err(`expect.${lang}.output is true but ${files.output} does not exist`);
    if (!exp.output && hasOutput) err(`expect.${lang}.output is false but ${files.output} exists`);
    if (exp.output && exp.diagnostics.length === 0) {
      err(`expect.${lang}.output is true but there are no diagnostics`);
    }
    if (exp.output && hasInput && hasOutput && sameBytes(inputPath, outputPath)) {
      err(`expect.${lang}.output is true but ${files.output} is identical to ${files.input}`);
    }
    exp.diagnostics.forEach((diag, i) => {
      const at = `expect.${lang}.diagnostics[${i}]`;
      if (diag.ruleId !== d.rule) err(`${at} ruleId "${diag.ruleId}" must equal the case rule "${d.rule}"`);
      if (diag.skip !== undefined && exp.output) err(`${at} has skip "${diag.skip}" but output is true`);
      if (!rule) return;
      if (rule.skip !== undefined) {
        if (diag.skip !== rule.skip) {
          err(`${at} must carry skip "${rule.skip}" (rule "${rule.id}" is report-only), got ${diag.skip === undefined ? "none" : `"${diag.skip}"`}`);
        }
      } else if (diag.skip !== undefined) {
        err(`${at} has skip "${diag.skip}" but rule "${rule.id}" is a rewriting rule; diagnostics of rewriting rules carry no skip`);
      }
    });
  }
  return errors;
}
