import { describe, expect, it } from "vitest";
import { discoverCases } from "../src/load.js";
import { resolveSpecRoot } from "../src/specRoot.js";
import { loadRules } from "../src/load.js";
import { validateCaseSemantics, validateRulesSemantics } from "../src/semantic.js";
import { loadValidators, validateCase } from "../src/validate.js";

const root = resolveSpecRoot();
const cases = discoverCases(root);
const validators = loadValidators(root);

describe(`conformance (spec root: ${root})`, () => {
  it("has the fixture schema available", () => {
    expect(validators.fixture, "schema/fixture.schema.json is required").toBeDefined();
  });

  it("has cross-consistent rules.yaml", () => {
    expect(validateRulesSemantics(loadRules(root))).toEqual([]);
  });

  for (const c of cases) {
    describe(c.relPath, () => {
      it("is a valid case.yaml", () => {
        expect(validators.fixture).toBeDefined();
        expect(validateCase(validators.fixture!, c)).toBeUndefined();
      });

      it("references known rules and has the promised fixture files", () => {
        expect(validateCaseSemantics(loadRules(root), c)).toEqual([]);
      });
      // JS expectations (diagnostics, output, idempotency) are asserted against
      // the real engine in packages/core/test/conformance.test.ts; spec must not
      // depend on core.
    });
  }
});
