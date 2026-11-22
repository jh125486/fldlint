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
  it("preserves a BOM", () => {
    const src = "﻿" + readFileSync(join(root, "fixtures/bool-variation/static-key-double-quote/js/input.ts"), "utf8");
    const out = runCodemod(src, "b.ts").output;
    expect(out?.startsWith("﻿")).toBe(true);
  });
});
