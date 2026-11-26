import { execFileSync, spawnSync } from "node:child_process";
import { cpSync, mkdirSync, readFileSync, mkdtempSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { beforeAll, describe, expect, it } from "vitest";
import { checkTags, MANDATORY_TAGS } from "../src/tags.js";
import { STUB_SPEC_ROOT } from "../src/specRoot.js";
import { validateSpec } from "../src/validate.js";

const RULES = `version: 1
skipCategories:
  detail: {severity: warning, message: "Detail call"}
  dynamic-key: {severity: warning, message: "Flag key is not a static string"}
  default-type-unproven: {severity: warning, message: "Default value type not proven"}
  receiver-unproven: {severity: warning, message: "LaunchDarkly client expression cannot be removed safely"}
rules:
  - id: bool-variation
    valueType: boolean
    risk: low
    keyArg: 0
    defaultArg: 2
    contextArg: 1
    languages:
      js: {methods: [boolVariation], template: "{{client}}.getBooleanValue({{key}}, {{default}}, {{context}})"}
      go: {methods: [BoolVariation], template: "{{client}}.BooleanValue({{goctx}}, {{key}}, {{default}}, {{evalctx}})"}
  - id: bool-variation-detail
    risk: high
    skip: detail
    languages:
      js: {methods: [boolVariationDetail]}
      go: {methods: [BoolVariationDetail]}
  - id: js-only
    risk: low
    languages:
      js: {methods: [jsOnly], template: "{{client}}.x({{key}})"}
`;

function w(root: string, rel: string, body: string): void {
  const p = join(root, rel);
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, body);
}

function caseYaml(id: string, rule: string, expect: string, tags = "[]"): string {
  return `id: ${id}\nrule: ${rule}\ntags: ${tags}\ndescription: d\nexpect:\n${expect}\n`;
}

const D = (extra = ""): string => `{line: 3, col: 18, ruleId: bool-variation${extra}}`;

/** A fully valid base spec with js+go templates, a skip rule and a js-only rule. */
function mkSpec(): string {
  const root = mkdtempSync(join(tmpdir(), "fldlint-neg-"));
  cpSync(join(STUB_SPEC_ROOT, "schema"), join(root, "schema"), { recursive: true });
  w(root, "rules.yaml", RULES);
  const f = "fixtures/bool-variation";
  w(root, `${f}/rw/case.yaml`, caseYaml("rw", "bool-variation", `  js: {diagnostics: [${D()}], output: true}\n  go: {diagnostics: [${D()}], output: true}`));
  w(root, `${f}/rw/js/input.ts`, "a\n");
  w(root, `${f}/rw/js/output.ts`, "b\n");
  w(root, `${f}/rw/go/input.go`, "a\n");
  w(root, `${f}/rw/go/output.go.golden`, "b\n");
  const g = "fixtures/bool-variation-detail";
  const dd = "{line: 3, col: 1, ruleId: bool-variation-detail, skip: detail}";
  w(root, `${g}/detail/case.yaml`, caseYaml("detail", "bool-variation-detail", `  js: {diagnostics: [${dd}], output: false}\n  go: {diagnostics: [${dd}], output: false}`));
  w(root, `${g}/detail/js/input.ts`, "a\n");
  w(root, `${g}/detail/go/input.go`, "a\n");
  const h = "fixtures/js-only";
  w(root, `${h}/rw/case.yaml`, caseYaml("js-only-rw", "js-only", "  js: {diagnostics: [{line: 1, col: 1, ruleId: js-only}], output: true}"));
  w(root, `${h}/rw/js/input.ts`, "a\n");
  w(root, `${h}/rw/js/output.ts`, "b\n");
  return root;
}

const REAL_SPEC = resolve(dirname(fileURLToPath(import.meta.url)), "../../../../spec");

const errs = (root: string): string => validateSpec(root).errors.join("\n");

describe("base spec", () => {
  it("is valid", () => {
    expect(validateSpec(mkSpec()).errors).toEqual([]);
  });
});

describe("template placeholders (real schema patterns)", () => {
  const defs = (JSON.parse(readFileSync(join(REAL_SPEC, "schema/rules.schema.json"), "utf8")) as { $defs: Record<string, { pattern: string }> }).$defs;
  const re = (name: string): RegExp => new RegExp(defs[name]!.pattern, "u");
  const cases: { name: string; tmpl: string; ok: boolean }[] = [
    { name: "templateJsGo", tmpl: "{{client}}.x({{key}})", ok: true },
    { name: "templateJsGo", tmpl: "}", ok: false },
    { name: "templateJsGo", tmpl: "{{client}}}", ok: false },
    { name: "templateJsGo", tmpl: "{{ key }}", ok: false },
    { name: "templateJava", tmpl: "#{key}", ok: true },
    { name: "templateJava", tmpl: "#{key}}", ok: false },
    { name: "templateJava", tmpl: "}", ok: false },
    { name: "message", tmpl: "for {{key}}", ok: true },
    { name: "message", tmpl: "}", ok: false },
  ];
  for (const c of cases) {
    it(`${c.name} ${c.ok ? "accepts" : "rejects"} ${JSON.stringify(c.tmpl)}`, () => {
      expect(re(c.name).test(c.tmpl)).toBe(c.ok);
    });
  }
});

describe("fixture discovery", () => {
  it("rejects a case directory with no case.yaml", () => {
    const r = mkSpec();
    rmSync(join(r, "fixtures/bool-variation/rw/case.yaml"));
    expect(errs(r)).toContain("fixtures/bool-variation/rw: missing case.yaml");
  });

  it("rejects case.yml and says so", () => {
    const r = mkSpec();
    renameSync(join(r, "fixtures/bool-variation/rw/case.yaml"), join(r, "fixtures/bool-variation/rw/case.yml"));
    const e = errs(r);
    expect(e).toContain("missing case.yaml");
    expect(e).toContain("found case.yml");
  });

  it("rejects a case nested one level too deep", () => {
    const r = mkSpec();
    mkdirSync(join(r, "fixtures/bool-variation/group"), { recursive: true });
    renameSync(join(r, "fixtures/bool-variation/rw"), join(r, "fixtures/bool-variation/group/rw"));
    const e = errs(r);
    expect(e).toContain("fixtures/bool-variation/group: missing case.yaml");
    expect(e).toContain("unexpected directory in case directory");
  });

  it("rejects a nested directory inside a valid case", () => {
    const r = mkSpec();
    w(r, "fixtures/bool-variation/rw/extra/case.yaml", "x: 1\n");
    expect(errs(r)).toContain("fixtures/bool-variation/rw/extra: unexpected directory in case directory");
  });

  it("rejects stray files at fixtures/ and fixtures/<rule>/ level", () => {
    const r = mkSpec();
    w(r, "fixtures/notes.txt", "x");
    w(r, "fixtures/bool-variation/notes.txt", "x");
    const e = errs(r);
    expect(e).toContain("fixtures/notes.txt: stray file at fixtures/ level");
    expect(e).toContain("fixtures/bool-variation/notes.txt: stray file at fixtures/<rule>/ level");
  });

  it("rejects a misnamed output file and a stray language dir without expect", () => {
    const r = mkSpec();
    w(r, "fixtures/bool-variation/rw/go/output.go", "x");
    expect(errs(r)).toContain("unexpected entry in go/");
    const r2 = mkSpec();
    w(r2, "fixtures/js-only/rw/go/input.go", "x");
    expect(errs(r2)).toContain("go/input.go");
  });

  it("rejects an unknown dir, a stray file inside a case, and a stray language-dir file", () => {
    const r = mkSpec();
    w(r, "fixtures/bool-variation/rw/notes.txt", "x");
    expect(errs(r)).toContain("fixtures/bool-variation/rw/notes.txt: unexpected file in case directory");
    const r2 = mkSpec();
    w(r2, "fixtures/bool-variation/rw/js/extra.ts", "x");
    expect(errs(r2)).toContain("unexpected entry in js/");
  });

  it("rejects a missing fixtures directory", () => {
    const r = mkSpec();
    rmSync(join(r, "fixtures"), { recursive: true });
    expect(errs(r)).toContain("fixtures directory not found");
  });

  it("rejects duplicate case ids across the spec", () => {
    const r = mkSpec();
    w(r, "fixtures/js-only/rw/case.yaml", caseYaml("rw", "js-only", "  js: {diagnostics: [{line: 1, col: 1, ruleId: js-only}], output: true}"));
    expect(errs(r)).toContain('duplicate case id "rw"');
  });
});

describe("case semantics", () => {
  const set = (r: string, rel: string, expect: string, rule = "bool-variation", id = "rw"): void =>
    w(r, rel, caseYaml(id, rule, expect));

  it("rejects diagnostics[].ruleId that differs from the case rule (another valid rule)", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [{line: 3, col: 18, ruleId: js-only}], output: true}`);
    expect(errs(r)).toContain('ruleId "js-only" must equal the case rule "bool-variation"');
  });

  it("rejects a wrong skip category on a skip rule", () => {
    const r = mkSpec();
    const bad = "{line: 3, col: 1, ruleId: bool-variation-detail, skip: dynamic-key}";
    set(r, "fixtures/bool-variation-detail/detail/case.yaml", `  js: {diagnostics: [${bad}], output: false}`, "bool-variation-detail", "detail");
    expect(errs(r)).toContain('must carry skip "detail"');
  });

  it("requires a skip rule's diagnostics to carry skip", () => {
    const r = mkSpec();
    const bad = "{line: 3, col: 1, ruleId: bool-variation-detail}";
    set(r, "fixtures/bool-variation-detail/detail/case.yaml", `  js: {diagnostics: [${bad}], output: false}`, "bool-variation-detail", "detail");
    expect(errs(r)).toContain('must carry skip "detail"');
  });

  it("rejects a rule-level skip (detail) on a rewriting rule", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [${D(", skip: detail")}], output: false}`);
    expect(errs(r)).toContain("only allows call-site skips");
  });

  it("accepts a call-site skip (dynamic-key) on a rewriting rule with output false", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [${D(", skip: dynamic-key")}], output: false}`);
    rmSync(join(r, "fixtures/bool-variation/rw/js/output.ts"));
    rmSync(join(r, "fixtures/bool-variation/rw/go"), { recursive: true });
    expect(errs(r)).toBe("");
  });

  it("accepts the default-type-unproven call-site skip on a rewriting rule with output false", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [${D(", skip: default-type-unproven")}], output: false}`);
    rmSync(join(r, "fixtures/bool-variation/rw/js/output.ts"));
    rmSync(join(r, "fixtures/bool-variation/rw/go"), { recursive: true });
    expect(errs(r)).toBe("");
  });

  it("accepts the receiver-unproven call-site skip on a rewriting rule with output false", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [${D(", skip: receiver-unproven")}], output: false}`);
    rmSync(join(r, "fixtures/bool-variation/rw/js/output.ts"));
    rmSync(join(r, "fixtures/bool-variation/rw/go"), { recursive: true });
    expect(errs(r)).toBe("");
  });

  it("rejects an unknown skip category on a rewriting rule", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [${D(", skip: not-a-category")}], output: false}`);
    expect(errs(r)).toContain("not-a-category");
  });

  it("rejects a no-skip diagnostic on a rewriting rule when output is false", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", `  js: {diagnostics: [${D()}], output: false}`);
    expect(errs(r)).toContain("has no skip on rewriting rule");
  });

  it("rejects skip together with output: true", () => {
    const r = mkSpec();
    const d = "{line: 3, col: 1, ruleId: bool-variation-detail, skip: detail}";
    set(r, "fixtures/bool-variation-detail/detail/case.yaml", `  js: {diagnostics: [${d}], output: true}`, "bool-variation-detail", "detail");
    w(r, "fixtures/bool-variation-detail/detail/js/output.ts", "b\n");
    expect(errs(r)).toContain('has skip "detail" but output is true');
  });

  it("rejects output: true with empty diagnostics", () => {
    const r = mkSpec();
    set(r, "fixtures/bool-variation/rw/case.yaml", "  js: {diagnostics: [], output: true}");
    expect(errs(r)).toContain("output is true but there are no diagnostics");
  });

  it("rejects an output file identical to the input", () => {
    const r = mkSpec();
    w(r, "fixtures/bool-variation/rw/js/output.ts", "a\n");
    expect(errs(r)).toContain("js/output.ts is identical to js/input.ts");
  });

  it("rejects expect.<lang> when the rule does not bind that language", () => {
    const r = mkSpec();
    set(r, "fixtures/js-only/rw/case.yaml", `  js: {diagnostics: [{line: 1, col: 1, ruleId: js-only}], output: true}\n  go: {diagnostics: [], output: false}`, "js-only", "js-only-rw");
    w(r, "fixtures/js-only/rw/go/input.go", "a\n");
    expect(errs(r)).toContain('expect.go is not allowed: rule "js-only" does not bind language go');
  });
});

describe("spec:tags coverage", () => {
  function withShapeTags(r: string): void {
    for (const t of MANDATORY_TAGS) {
      w(r, `fixtures/bool-variation/t-${t}/case.yaml`, caseYaml(`t-${t}`, "bool-variation", "  js: {diagnostics: [], output: false}", `[${t}]`));
      w(r, `fixtures/bool-variation/t-${t}/js/input.ts`, "a\n");
    }
  }

  it("passes on a spec with a rewrite case per templated language", () => {
    const r = mkSpec();
    withShapeTags(r);
    expect(checkTags(r).errors).toEqual([]);
  });

  it("requires a rewrite case for each language with a template", () => {
    const r = mkSpec();
    withShapeTags(r);
    rmSync(join(r, "fixtures/bool-variation/rw/go"), { recursive: true });
    w(r, "fixtures/bool-variation/rw/case.yaml", caseYaml("rw", "bool-variation", `  js: {diagnostics: [${D()}], output: true}`));
    expect(checkTags(r).errors.join("\n")).toContain('rule "bool-variation": no rewrite case for language "go"');
  });

  it("is not satisfied by an empty-diagnostics rewrite case", () => {
    const r = mkSpec();
    withShapeTags(r);
    w(r, "fixtures/js-only/rw/case.yaml", caseYaml("js-only-rw", "js-only", "  js: {diagnostics: [], output: true}"));
    expect(checkTags(r).errors.join("\n")).toContain('rule "js-only": no rewrite case for language "js"');
  });

  it("is not satisfied by an identical-output case", () => {
    const r = mkSpec();
    withShapeTags(r);
    w(r, "fixtures/js-only/rw/js/output.ts", "a\n");
    expect(checkTags(r).errors.join("\n")).toContain('rule "js-only": no rewrite case for language "js"');
  });

  it("requires a java rewrite case for a java template", () => {
    const r = mkSpec();
    withShapeTags(r);
    w(r, "rules.yaml", RULES.replace('go: {methods: [BoolVariation],', 'java: {methods: [boolVariation], template: "#{key}"}\n      go: {methods: [BoolVariation],'));
    expect(checkTags(r).errors.join("\n")).toContain('rule "bool-variation": no rewrite case for language "java"');
  });

  it("is satisfied by a java rewrite case", () => {
    const r = mkSpec();
    withShapeTags(r);
    w(r, "rules.yaml", RULES.replace('go: {methods: [BoolVariation],', 'java: {methods: [boolVariation], template: "#{key}"}\n      go: {methods: [BoolVariation],'));
    w(r, "fixtures/bool-variation/rw/case.yaml", caseYaml("rw", "bool-variation", `  js: {diagnostics: [${D()}], output: true}\n  go: {diagnostics: [${D()}], output: true}\n  java: {diagnostics: [${D()}], output: true}`));
    w(r, "fixtures/bool-variation/rw/java/Before.java", "a\n");
    w(r, "fixtures/bool-variation/rw/java/After.java", "b\n");
    expect(checkTags(r).errors).toEqual([]);
  });

  it("does not ask for a java case from a java report-only binding", () => {
    const r = mkSpec();
    withShapeTags(r);
    w(r, "rules.yaml", RULES.replace("js: {methods: [boolVariationDetail]}", "js: {methods: [boolVariationDetail]}\n      java: {methods: [boolVariationDetail]}"));
    expect(checkTags(r).errors).toEqual([]);
  });

  it("is not satisfied by a rewrite case whose diagnostic carries a skip", () => {
    const r = mkSpec();
    withShapeTags(r);
    w(r, "fixtures/js-only/rw/case.yaml", caseYaml("js-only-rw", "js-only", "  js: {diagnostics: [{line: 1, col: 1, ruleId: js-only, skip: dynamic-key}], output: true}"));
    expect(checkTags(r).errors.join("\n")).toContain('rule "js-only": no rewrite case for language "js"');
  });

  it("does not ask for skip cases and ignores skip rules", () => {
    const r = mkSpec();
    withShapeTags(r);
    rmSync(join(r, "fixtures/bool-variation-detail"), { recursive: true });
    const e = checkTags(r).errors.join("\n");
    expect(e).not.toContain("skip");
    expect(e).not.toContain("bool-variation-detail");
  });

  it("reports a missing rules.yaml instead of throwing", () => {
    const r = mkSpec();
    rmSync(join(r, "rules.yaml"));
    expect(checkTags(r).errors[0]).toContain("rules.yaml");
  });

  it("names every missing shape tag", () => {
    const r = mkSpec();
    const e = checkTags(r).errors;
    for (const t of MANDATORY_TAGS) expect(e).toContain(`rule "bool-variation": missing mandatory tag "${t}"`);
  });
});

describe("bins: --spec-root handling", () => {
  const pkg = resolve(dirname(fileURLToPath(import.meta.url)), "..");
  const tsc = resolve(pkg, "../../node_modules/typescript/bin/tsc");
  beforeAll(() => {
    execFileSync(process.execPath, [tsc, "-p", join(pkg, "tsconfig.build.json")], { cwd: pkg });
  }, 120_000);
  const run = (
    bin: string,
    args: string[],
    specRootEnv = "",
  ): { status: number | null; stderr: string; stdout: string } => {
    const r = spawnSync(process.execPath, [join(pkg, "dist/bin", `${bin}.js`), ...args], {
      encoding: "utf8",
      env: { ...process.env, FLDLINT_SPEC_ROOT: specRootEnv },
    });
    return { status: r.status, stderr: r.stderr, stdout: r.stdout };
  };

  for (const bin of ["validate", "tags"]) {
    it(`${bin}: missing explicit root exits 2, no stub fallback`, () => {
      const r = run(bin, ["--spec-root=/no/such/root"]);
      expect(r.status).toBe(2);
      expect(r.stderr).toContain("missing or invalid");
      expect(r.stdout).not.toContain("OK");
    });

    it(`${bin}: explicit root lacking rules.yaml or schema exits 2`, () => {
      const r1 = mkSpec();
      rmSync(join(r1, "rules.yaml"));
      expect(run(bin, ["--spec-root", r1]).status).toBe(2);
      const r2 = mkSpec();
      rmSync(join(r2, "schema"), { recursive: true });
      const out = run(bin, [`--spec-root=${r2}`, ...(bin === "validate" ? ["--allow-missing-schema"] : [])]);
      expect(out.status).toBe(2);
    });

    it(`${bin}: unknown arguments exit 2`, () => {
      expect(run(bin, ["--spec-rot=/x"]).status).toBe(2);
    });
  }

  it("validate: with no spec root the built CLI uses the repo spec, not the stub", () => {
    const r = run("validate", []);
    expect(r.status).toBe(0);
    expect(r.stderr).not.toContain("stub");
    const m = /root=(.*), cases checked=(\d+)/.exec(r.stdout);
    expect(m).not.toBeNull();
    expect(m![1]).toBe(resolve(pkg, "../../../spec"));
    expect(Number(m![2])).toBeGreaterThanOrEqual(21);
  });

  it("gen: an invalid FLDLINT_SPEC_ROOT exits 2 and writes nothing", () => {
    const r = run("gen", [], "/no/such/root");
    expect(r.status).toBe(2);
    expect(r.stderr).toContain("missing or invalid");
    expect(r.stdout).not.toContain("wrote");
  });

  it("validate: a valid explicit root passes", () => {
    expect(run("validate", [`--spec-root=${mkSpec()}`]).status).toBe(0);
  });
});
