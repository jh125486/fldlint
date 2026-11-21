import { describe, expect, it } from "vitest";
import { discoverCases } from "../src/load.js";
import { resolveSpecRoot } from "../src/specRoot.js";
import { loadValidators, validateCase } from "../src/validate.js";

const root = resolveSpecRoot();
const cases = discoverCases(root);
const validators = loadValidators(root);

describe(`conformance (spec root: ${root})`, () => {
  it("has the fixture schema available", () => {
    expect(validators.fixture, "schema/fixture.schema.json is required").toBeDefined();
  });

  for (const c of cases) {
    describe(c.relPath, () => {
      it("is a valid case.yaml", () => {
        expect(validators.fixture).toBeDefined();
        expect(validateCase(validators.fixture!, c)).toBeUndefined();
      });

      const expectMap = (c.data as { expect?: Record<string, unknown> } | undefined)?.expect ?? {};
      if (Object.hasOwn(expectMap, "js")) {
        // Pending until the JS detection engine lands (M1).
        it.skip("js expectation (engine not implemented)", () => {});
      }
    });
  }
});
