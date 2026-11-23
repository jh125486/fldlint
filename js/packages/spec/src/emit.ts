import { generateSpecJson, type SpecJsonCase } from "./gen.js";
import { validateSpec } from "./validate.js";

/** Thrown when the spec does not validate, so nothing is emitted from it. */
export class EmitError extends Error {}

/**
 * Validate the whole spec (YAML subset, JSON Schema, semantic checks) and
 * return the canonical spec.json text. Throws EmitError, listing every
 * problem, when anything fails: invalid input never produces output.
 */
export function emitSpecJson(specRoot: string): string {
  const res = validateSpec(specRoot);
  const problems = [...res.errors, ...res.missingSchemas.map((s) => `schema file not found: ${s}`)];
  if (problems.length > 0 || res.rulesFile === undefined) {
    throw new EmitError(`spec does not validate, nothing emitted:\n  ${problems.join("\n  ")}`);
  }
  const cases: SpecJsonCase[] = res.cases.map((c) => ({
    path: c.relPath.replace(/^fixtures\//, "").replace(/\/case\.yaml$/, ""),
    case: c.data,
  }));
  return generateSpecJson(res.rulesFile, cases);
}
