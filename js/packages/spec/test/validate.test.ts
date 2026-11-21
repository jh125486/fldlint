import { cpSync, mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { discoverCases } from "../src/load.js";
import { resolveSpecRoot, STUB_SPEC_ROOT } from "../src/specRoot.js";
import { loadValidators, validateCase, validateSpec } from "../src/validate.js";

function copyStub(): string {
  const dir = mkdtempSync(join(tmpdir(), "fldlint-spec-"));
  cpSync(STUB_SPEC_ROOT, dir, { recursive: true });
  return dir;
}

describe("resolveSpecRoot", () => {
  it("prefers explicit arg, then env", () => {
    expect(resolveSpecRoot("/a", { FLDLINT_SPEC_ROOT: "/b" })).toBe("/a");
    expect(resolveSpecRoot(undefined, { FLDLINT_SPEC_ROOT: "/b" })).toBe("/b");
  });
});

describe("discoverCases", () => {
  it("finds the stub cases sorted", () => {
    expect(discoverCases(STUB_SPEC_ROOT).map((c) => c.relPath)).toEqual([
      "fixtures/bool-variation/dynamic-key/case.yaml",
      "fixtures/bool-variation/static-key-double-quote/case.yaml",
    ]);
  });
  it("returns [] when there is no fixtures dir", () => {
    expect(discoverCases(mkdtempSync(join(tmpdir(), "fldlint-empty-")))).toEqual([]);
  });
});

describe("validateSpec", () => {
  it("passes on the stub spec", () => {
    const r = validateSpec(STUB_SPEC_ROOT);
    expect(r.errors).toEqual([]);
    expect(r.missingSchemas).toEqual([]);
    expect(r.casesChecked).toBe(2);
  });

  it("fails on a malformed case", () => {
    const dir = copyStub();
    const bad = join(dir, "fixtures/bool-variation/bad");
    mkdirSync(bad, { recursive: true });
    writeFileSync(join(bad, "case.yaml"), "id: bad\nrule: bool-variation\n");
    const r = validateSpec(dir);
    expect(r.errors).toHaveLength(1);
    expect(r.errors[0]).toContain("fixtures/bool-variation/bad/case.yaml");
  });

  it("fails on unparseable YAML", () => {
    const dir = copyStub();
    const bad = join(dir, "fixtures/bool-variation/broken");
    mkdirSync(bad, { recursive: true });
    writeFileSync(join(bad, "case.yaml"), "id: [unclosed\n");
    const v = loadValidators(dir).fixture!;
    const c = discoverCases(dir).find((x) => x.relPath.includes("broken"))!;
    expect(validateCase(v, c)).toContain("YAML parse error");
  });

  it("fails on invalid rules.yaml", () => {
    const dir = copyStub();
    writeFileSync(join(dir, "rules.yaml"), "version: 2\nrules: []\n");
    expect(validateSpec(dir).errors[0]).toContain("rules.yaml");
  });

  it("reports missing schemas", () => {
    const dir = mkdtempSync(join(tmpdir(), "fldlint-noschema-"));
    writeFileSync(join(dir, "rules.yaml"), "version: 1\nskipCategories: {}\nrules: []\n");
    expect(validateSpec(dir).missingSchemas).toHaveLength(2);
  });
});
