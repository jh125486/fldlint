import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import AjvModule from "ajv/dist/2020.js";
import type { ValidateFunction } from "ajv/dist/2020.js";
import { discoverCases, loadYaml, type CaseFile } from "./load.js";
import { validateCaseSemantics, validateRulesSemantics } from "./semantic.js";
import type { RulesFile } from "./types.js";

// ajv is CommonJS; under NodeNext the constructor is on `.default`.
const Ajv2020 = AjvModule.default;

export const RULES_SCHEMA = "schema/rules.schema.json";
export const FIXTURE_SCHEMA = "schema/fixture.schema.json";

export interface Validators {
  rules: ValidateFunction | undefined;
  fixture: ValidateFunction | undefined;
}

/** Load whichever schemas exist under specRoot; missing ones are undefined. */
export function loadValidators(specRoot: string): Validators {
  const ajv = new Ajv2020({ allErrors: true, strict: false });
  const compile = (rel: string): ValidateFunction | undefined => {
    const p = join(specRoot, rel);
    if (!existsSync(p)) return undefined;
    return ajv.compile(JSON.parse(readFileSync(p, "utf8")) as object);
  };
  return { rules: compile(RULES_SCHEMA), fixture: compile(FIXTURE_SCHEMA) };
}

export function formatErrors(v: ValidateFunction): string {
  return (v.errors ?? []).map((e) => `${e.instancePath || "/"} ${e.message ?? "invalid"}`).join("; ");
}

/** Validate one discovered case; returns an error message or undefined when valid. */
export function validateCase(v: ValidateFunction, c: CaseFile): string | undefined {
  if (c.parseError !== undefined) return `${c.relPath}: YAML parse error: ${c.parseError}`;
  if (!v(c.data)) return `${c.relPath}: ${formatErrors(v)}`;
  return undefined;
}

export interface ValidateResult {
  errors: string[];
  missingSchemas: string[];
  casesChecked: number;
}

export function validateSpec(specRoot: string): ValidateResult {
  const errors: string[] = [];
  const missingSchemas: string[] = [];
  const v = loadValidators(specRoot);
  let rulesFile: RulesFile | undefined;
  if (v.rules) {
    try {
      const data = loadYaml(join(specRoot, "rules.yaml"));
      if (!v.rules(data)) errors.push(`rules.yaml: ${formatErrors(v.rules)}`);
      else {
        rulesFile = data as RulesFile;
        errors.push(...validateRulesSemantics(rulesFile));
      }
    } catch (e) {
      errors.push(`rules.yaml: ${(e as Error).message}`);
    }
  } else missingSchemas.push(RULES_SCHEMA);
  const cases = discoverCases(specRoot);
  if (v.fixture) {
    for (const c of cases) {
      const err = validateCase(v.fixture, c);
      if (err) errors.push(err);
      else if (rulesFile) errors.push(...validateCaseSemantics(rulesFile, c));
    }
  } else missingSchemas.push(FIXTURE_SCHEMA);
  return { errors, missingSchemas, casesChecked: v.fixture ? cases.length : 0 };
}
