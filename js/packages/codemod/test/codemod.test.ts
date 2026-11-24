import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { describe, expect, it } from "vitest";
import { discoverCases, resolveSpecRoot } from "@fldlint/spec";
import { rewrite } from "@fldlint/core";
import { runCodemod } from "../src/run.js";

type Transform = (sg: { filename(): string; root(): { text(): string } }) => string | null;
// The bundle is what `npx codemod` executes (built by `pretest`).
const bundle = (await import(pathToFileURL(join(import.meta.dirname, "../dist/transform.js")).href)) as { default: Transform };

function fake(source: string, name = "input.ts") {
  return { filename: () => name, root: () => ({ text: () => source }) };
}

const root = resolveSpecRoot();
const cases = discoverCases(root).filter((c) => existsSync(join(c.dir, "js", "input.ts")));

describe("shared JS fixtures through the bundled transform", () => {
  it("finds fixtures", () => expect(cases.length).toBeGreaterThan(20));
  for (const c of cases) {
    const dir = join(c.dir, "js");
    const input = readFileSync(join(dir, "input.ts"), "utf8");
    const hasOutput = existsSync(join(dir, "output.ts"));
    const expected = (c.data as { expect?: { js?: { diagnostics: { line: number; col: number; skip?: string }[] } } }).expect?.js;
    describe(c.relPath, () => {
      if (hasOutput) {
        const output = readFileSync(join(dir, "output.ts"), "utf8");
        it("rewrites to output.ts", () => {
          expect(bundle.default(fake(input))).toBe(output);
          expect(runCodemod(input, "input.ts").output).toBe(output);
        });
        it("is idempotent", () => expect(bundle.default(fake(output))).toBeNull());
        it("matches core rewrite()", () => expect(output).toBe(rewrite(input)));
      } else {
        it("returns null (file untouched)", () => expect(bundle.default(fake(input))).toBeNull());
      }
      it("reports every skip with its category", () => {
        const { reports } = runCodemod(input, "input.ts");
        const want = (expected?.diagnostics ?? []).filter((d) => d.skip);
        expect(reports).toHaveLength(want.length);
        want.forEach((d, i) => {
          expect(reports[i]).toMatch(new RegExp(`^input\\.ts:${d.line}:${d.col}  \\S+  .+ \\[skip: ${d.skip}\\]$`));
        });
      });
    });
  }
});

describe("comments inside a call (issue #31)", () => {
  const head = `import { init } from "@launchdarkly/node-server-sdk";\nimport { OpenFeature } from "@openfeature/server-sdk";\nconst ld = init("x");\nconst c = OpenFeature.getClient();\n`;
  it("moves a comment with its argument", () => {
    const out = runCodemod(`${head}export const v = ld.boolVariation("k", /* who */ { key: u }, false);\n`, "a.ts");
    expect(out.output).toBe(`${head}export const v = c.getBooleanValue("k", false, /* who */ { targetingKey: u });\n`);
    expect(out.reports).toEqual([]);
  });
  it("leaves a call whose comment has no home untouched and reports it", () => {
    const src = `${head}export const v = ld.boolVariation("k", { key: u /* id */ }, false);\n`;
    const out = runCodemod(src, "a.ts");
    expect(out.output).toBeNull();
    expect(out.reports).toEqual([`a.ts:5:18  bool-variation  call left unchanged: a comment inside it has no home in the rewritten call [no-fix: comment]`]);
  });
});

describe("runCodemod", () => {
  it("leaves unparseable files untouched and reports them", () => {
    const r = runCodemod("const = ;", "bad.ts");
    expect(r.output).toBeNull();
    expect(r.reports).toHaveLength(1);
    expect(r.reports[0]).toMatch(/^bad\.ts {2}parse-error {2}/);
  });
  it("never rewrites the legacy launchdarkly-node-server-sdk package", () => {
    const src = `import * as ld from "launchdarkly-node-server-sdk";\nconst c = ld.init("k");\nawait c.boolVariation("f", { key: "u" }, false);\n`;
    expect(runCodemod(src, "legacy.ts").output).toBeNull();
  });
  it("handles .tsx through the file name", () => {
    const src = readFileSync(join(root, "fixtures/bool-variation/static-key-double-quote/js/input.ts"), "utf8") + "\nexport const el = <div />;\n";
    expect(bundle.default(fake(src, "a.tsx"))).toBe(rewrite(src, "a.tsx"));
  });
  describe("BOM", () => {
    const fx = (name: string) => readFileSync(join(root, "fixtures", name, "js", "input.ts"), "utf8");
    for (const name of ["bool-variation/static-key-double-quote", "bool-variation/dynamic-key", "bool-variation/stranded-two-calls"]) {
      it(`${name}: BOM file == BOM + rewrite(no-BOM), idempotent, positions unchanged`, () => {
        const plain = fx(name);
        const bom = "\uFEFF" + plain;
        const want = rewrite(plain);
        const got = runCodemod(bom, "b.ts");
        expect(got.output).toBe(want === plain ? null : "\uFEFF" + want);
        expect(got.reports).toEqual(runCodemod(plain, "b.ts").reports);
        expect(bundle.default(fake(bom, "b.ts"))).toBe(got.output);
        if (got.output !== null) expect(runCodemod(got.output, "b.ts").output).toBeNull();
      });
    }
  });
  describe("deeply nested rewrites", () => {
    const head = `import { init } from "@launchdarkly/node-server-sdk";\nimport { OpenFeature } from "@openfeature/server-sdk";\nconst client = OpenFeature.getClient();\nconst ld = init("k");\nconst u = "u";\n`;
    const nested = (depth: number): string => {
      let expr = "u";
      for (let i = 0; i < depth; i++) expr = `ld.boolVariation("f${i}", { key: ${expr} }, false) ? "x" : "y"`;
      return `${head}export const v = ${expr};\n`;
    };
    it("rewrites 12 levels completely (more than ten passes)", () => {
      const out = runCodemod(nested(12), "deep.ts");
      expect(out.reports).toEqual([]);
      expect(out.output).not.toBeNull();
      expect(out.output).not.toContain("boolVariation");
    });
    it("leaves the file untouched and reports when the pass cap is hit", () => {
      const out = runCodemod(nested(12), "deep.ts", 3);
      expect(out.output).toBeNull();
      expect(out.reports).toHaveLength(1);
      expect(out.reports[0]).toMatch(/^deep\.ts {2}max-passes {2}.*left unchanged/);
    });
  });
});
