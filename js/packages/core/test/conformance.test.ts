import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { discoverCases, resolveSpecRoot } from "@fldlint/spec";
import { analyze, rewrite } from "../src/index.js";

const root = resolveSpecRoot();

interface Expected {
  diagnostics: { line: number; col: number; ruleId: string; skip?: string }[];
  output: boolean;
}

describe(`js conformance (spec root: ${root})`, () => {
  for (const c of discoverCases(root)) {
    const expected = (c.data as { expect?: { js?: Expected } } | undefined)?.expect?.js;
    if (!expected) continue;
    const dir = join(c.dir, "js");
    const input = readFileSync(join(dir, "input.ts"), "utf8");

    describe(c.relPath, () => {
      it("reports the expected diagnostics", () => {
        const got = analyze(input, "input.ts").map(({ site, result }) => ({
          line: site.line,
          col: site.col,
          ruleId: site.ruleId,
          ...(result.kind === "skip" && { skip: result.category }),
        }));
        expect(got).toEqual(expected.diagnostics.map((d) => ({ ...d })));
      });

      if (expected.output) {
        const output = readFileSync(join(dir, "output.ts"), "utf8");
        it("rewrites to the expected output", () => {
          expect(rewrite(input)).toBe(output);
        });
        it("is idempotent on the output", () => {
          expect(analyze(output)).toEqual([]);
          expect(rewrite(output)).toBe(output);
        });
      } else {
        it("leaves the source unchanged", () => {
          expect(existsSync(join(dir, "output.ts"))).toBe(false);
          expect(rewrite(input)).toBe(input);
        });
      }
    });
  }
});
