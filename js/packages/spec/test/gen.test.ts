import { cpSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { EmitError, emitSpecJson } from "../src/emit.js";
import { findRepoSpec } from "../src/specRoot.js";
import { GEN_HEADER, generateRulesTs, generateSpecJson, SPEC_JSON } from "../src/gen.js";
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

describe("spec.json", () => {
  const here = dirname(fileURLToPath(import.meta.url));
  const repoSpec = findRepoSpec(here);

  it("is byte-identical to a fresh emit of the repo spec", () => {
    expect(repoSpec).toBeDefined();
    const committed = readFileSync(join(repoSpec!, SPEC_JSON), "utf8");
    expect(emitSpecJson(repoSpec!)).toBe(committed);
  });

  it("keeps the generated Go stub spec.json in step with its YAML stub", () => {
    const stubRoot = join(repoSpec!, "../go/conformance/testdata");
    const committed = readFileSync(join(repoSpec!, "../go/internal/spec/testdata/spec.json"), "utf8");
    expect(emitSpecJson(stubRoot, repoSpec!)).toBe(committed);
  });

  it("is deterministic, ends in a newline and sorts cases by path", () => {
    const out = emitSpecJson(STUB_SPEC_ROOT);
    expect(emitSpecJson(STUB_SPEC_ROOT)).toBe(out);
    expect(out.endsWith("}\n")).toBe(true);
    const parsed = JSON.parse(out) as { rules: unknown; cases: { path: string }[] };
    const paths = parsed.cases.map((c) => c.path);
    expect(paths).toEqual([...paths].sort());
    expect(paths).toContain("bool-variation/dynamic-key");
  });

  it("keeps rules order and sorts shuffled cases", () => {
    const rf = loadRules(STUB_SPEC_ROOT);
    const out = JSON.parse(generateSpecJson(rf, [{ path: "b/x", case: {} }, { path: "a/y", case: {} }])) as {
      cases: { path: string }[];
    };
    expect(out.cases.map((c) => c.path)).toEqual(["a/y", "b/x"]);
  });

  it("refuses to emit when a case fails to parse or validate", () => {
    const bad: [string, string][] = [
      ["a YAML subset violation", "id: x\nanchor: &a 1\n"],
      ["a schema violation", "id: dynamic-key\nrule: bool-variation\n"],
    ];
    for (const [label, body] of bad) {
      const dir = mkdtempSync(join(tmpdir(), "fldlint-emit-"));
      cpSync(STUB_SPEC_ROOT, dir, { recursive: true });
      writeFileSync(join(dir, "fixtures/bool-variation/dynamic-key/case.yaml"), body);
      expect(() => emitSpecJson(dir), label).toThrow(EmitError);
    }
  });

  it("refuses to emit when a semantic check fails", () => {
    const dir = mkdtempSync(join(tmpdir(), "fldlint-emit-"));
    cpSync(STUB_SPEC_ROOT, dir, { recursive: true });
    writeFileSync(join(dir, "rules.yaml"), "version: 1\nskipCategories: {}\nrules: []\n");
    expect(() => emitSpecJson(dir)).toThrow(/nothing emitted/);
  });
});
