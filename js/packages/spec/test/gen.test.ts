import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { GEN_HEADER, generateRulesTs } from "../src/gen.js";
import { loadRules } from "../src/load.js";
import { STUB_SPEC_ROOT } from "../src/specRoot.js";
import type { RulesFile } from "../src/types.js";

describe("generateRulesTs", () => {
  const rf = loadRules(STUB_SPEC_ROOT);

  it("starts with the generated-code header", () => {
    expect(generateRulesTs(rf).startsWith(`${GEN_HEADER}\n`)).toBe(true);
  });

  it("is deterministic", () => {
    expect(generateRulesTs(rf)).toBe(generateRulesTs(structuredClone(rf)));
  });

  it("is independent of object key order in the input", () => {
    const a: RulesFile = {
      version: 1,
      skipCategories: { b: { severity: "warning", message: "B" }, a: { severity: "warning", message: "A" } },
      rules: [{ id: "x", risk: "low", languages: { js: { template: "t", methods: ["m"] } } }],
    };
    const b: RulesFile = {
      rules: [{ languages: { js: { methods: ["m"], template: "t" } }, risk: "low", id: "x" }],
      skipCategories: { a: { message: "A", severity: "warning" }, b: { message: "B", severity: "warning" } },
      version: 1,
    };
    expect(generateRulesTs(a)).toBe(generateRulesTs(b));
  });

  it("preserves rule order", () => {
    const f: RulesFile = {
      version: 1,
      skipCategories: {},
      rules: [
        { id: "z", risk: "low", languages: {} },
        { id: "a", risk: "low", languages: {} },
      ],
    };
    const out = generateRulesTs(f);
    expect(out.indexOf('"id": "z"')).toBeLessThan(out.indexOf('"id": "a"'));
  });

  it("loads rules from a custom spec root", () => {
    const dir = mkdtempSync(join(tmpdir(), "fldlint-"));
    writeFileSync(join(dir, "rules.yaml"), "version: 1\nskipCategories: {}\nrules: []\n");
    expect(loadRules(dir).rules).toEqual([]);
  });
});
