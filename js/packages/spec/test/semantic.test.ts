import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { describe, expect, it } from "vitest";
import { buildMethodIndex } from "../src/lookup.js";
import { checkTags, MANDATORY_TAGS } from "../src/tags.js";
import { STUB_SPEC_ROOT } from "../src/specRoot.js";
import { validateSpec } from "../src/validate.js";
import type { Rule } from "../src/types.js";

function copyStub(): string {
  const dir = mkdtempSync(join(tmpdir(), "fldlint-sem-"));
  cpSync(STUB_SPEC_ROOT, dir, { recursive: true });
  return dir;
}

function write(root: string, rel: string, body: string): void {
  const p = join(root, rel);
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, body);
}

const CASE = "fixtures/bool-variation/static-key-double-quote";

function caseYaml(over: { rule?: string; ruleId?: string; output?: boolean; langs?: string } = {}): string {
  return [
    "id: static-key-double-quote",
    `rule: ${over.rule ?? "bool-variation"}`,
    "tags: [quote-double]",
    "description: d",
    "expect:",
    `  js: {diagnostics: [{line: 3, col: 18, ruleId: ${over.ruleId ?? "bool-variation"}}], output: ${over.output ?? true}}`,
    over.langs ?? "",
  ].join("\n");
}

describe("semantic validation", () => {
  it("passes on the unmodified stub", () => {
    expect(validateSpec(copyStub()).errors).toEqual([]);
  });

  it("rejects a case whose rule is not in rules.yaml", () => {
    const dir = copyStub();
    write(dir, `${CASE}/case.yaml`, caseYaml({ rule: "no-such-rule" }));
    const errs = validateSpec(dir).errors.join("\n");
    expect(errs).toContain('rule "no-such-rule" is not defined in rules.yaml');
  });

  it("rejects a diagnostic ruleId that is not in rules.yaml", () => {
    const dir = copyStub();
    write(dir, `${CASE}/case.yaml`, caseYaml({ ruleId: "ghost-rule" }));
    expect(validateSpec(dir).errors.join("\n")).toContain('ruleId "ghost-rule" is not defined');
  });

  it("rejects expect.<lang> without the input file", () => {
    const dir = copyStub();
    write(dir, `${CASE}/case.yaml`, caseYaml({ langs: "  go: {diagnostics: [], output: false}" }));
    expect(validateSpec(dir).errors.join("\n")).toContain("expect.go requires go/input.go");
  });

  it("rejects output: true without the output file", () => {
    const dir = copyStub();
    rmSync(join(dir, CASE, "js/output.ts"));
    expect(validateSpec(dir).errors.join("\n")).toContain("output is true but js/output.ts does not exist");
  });

  it("rejects output: false when the output file exists", () => {
    const dir = copyStub();
    write(dir, `${CASE}/case.yaml`, caseYaml({ output: false }));
    expect(validateSpec(dir).errors.join("\n")).toContain("output is false but js/output.ts exists");
  });

  it("rejects an output file for a language with no expect entry", () => {
    const dir = copyStub();
    write(dir, `${CASE}/go/output.go.golden`, "package x\n");
    expect(validateSpec(dir).errors.join("\n")).toContain("go/output.go.golden exists but case.yaml has no expect.go");
  });

  it("rejects a case whose rule does not match its directory", () => {
    const dir = copyStub();
    cpSync(join(dir, CASE), join(dir, "fixtures/other-rule/c1"), { recursive: true });
    const rules = join(dir, "rules.yaml");
    writeFileSync(rules, `${cpSyncRead(rules)}  - id: other-rule\n    risk: low\n    skip: dynamic-key\n    languages:\n      js: {methods: [otherVariation]}\n`);
    expect(validateSpec(dir).errors.join("\n")).toContain('does not match its directory name "other-rule"');
  });

  it("rejects ctxArg keys that are not in methods", () => {
    const dir = copyStub();
    const rules = join(dir, "rules.yaml");
    writeFileSync(
      rules,
      cpSyncRead(rules).replace("js: {methods: [boolVariation],", "js: {methods: [boolVariation], ctxArg: {nope: 0},"),
    );
    expect(validateSpec(dir).errors.join("\n")).toContain('js.ctxArg key "nope" is not in js.methods');
  });

  it("rejects the same method in two rules for one language", () => {
    const dir = copyStub();
    const rules = join(dir, "rules.yaml");
    writeFileSync(
      rules,
      `${cpSyncRead(rules)}  - id: second\n    risk: low\n    skip: dynamic-key\n    languages:\n      js: {methods: [boolVariation]}\n`,
    );
    expect(validateSpec(dir).errors.join("\n")).toContain('js method "boolVariation" appears in rules "bool-variation" and "second"');
  });
});

function cpSyncRead(p: string): string {
  return readFileSync(p, "utf8");
}

describe("buildMethodIndex", () => {
  const mk = (id: string, m: string): Rule => ({ id, risk: "low", languages: { js: { methods: [m] } } });
  it("throws on duplicate methods instead of keeping the last", () => {
    expect(() => buildMethodIndex([mk("a", "m"), mk("b", "m")])).toThrow(/duplicate rule methods.*"a" and "b"/);
  });
  it("allows the same method name in different languages", () => {
    const go: Rule = { id: "g", risk: "low", languages: { go: { methods: ["m"] } } };
    expect(buildMethodIndex([mk("a", "m"), go]).size).toBe(2);
  });
});

describe("checkTags", () => {
  it("fails naming the rule and each missing mandatory tag", () => {
    const r = checkTags(STUB_SPEC_ROOT);
    for (const t of MANDATORY_TAGS.filter((t) => t !== "quote-double" && t !== "dynamic-key")) {
      expect(r.errors).toContain(`rule "bool-variation": missing mandatory tag "${t}"`);
    }
    expect(r.errors.join("\n")).not.toContain('"quote-double"');
  });

  it("passes when all seven tags and a rewrite are present", () => {
    const dir = copyStub();
    for (const t of MANDATORY_TAGS) {
      write(
        dir,
        `fixtures/bool-variation/t-${t}/case.yaml`,
        `id: t-${t}\nrule: bool-variation\ntags: [${t}]\ndescription: d\nexpect:\n  js: {diagnostics: [], output: false}\n`,
      );
    }
    expect(checkTags(dir).errors).toEqual([]);
  });

  it("requires a rewrite case for every other rewriting rule", () => {
    const dir = copyStub();
    const rules = join(dir, "rules.yaml");
    writeFileSync(
      rules,
      `${cpSyncRead(rules)}  - id: string-variation\n    risk: low\n    languages:\n      js: {methods: [stringVariation], template: "x"}\n`,
    );
    expect(checkTags(dir).errors).toContain('rule "string-variation": no case with a rewrite (expect.<lang>.output: true)');
  });

  it("does not require cases for skip rules", () => {
    const dir = copyStub();
    const rules = join(dir, "rules.yaml");
    writeFileSync(
      rules,
      `${cpSyncRead(rules)}  - id: detail\n    risk: high\n    skip: dynamic-key\n    languages:\n      js: {methods: [d]}\n`,
    );
    expect(checkTags(dir).errors.join("\n")).not.toContain('"detail"');
  });

  it("fails when fixtures are missing unless allowEmpty", () => {
    const dir = mkdtempSync(join(tmpdir(), "fldlint-nofix-"));
    expect(checkTags(dir).errors[0]).toContain("fixtures directory not found");
    const ok = checkTags(dir, { allowEmpty: true });
    expect(ok.errors).toEqual([]);
    expect(ok.note).toContain("allowed by --allow-empty");
  });
});
