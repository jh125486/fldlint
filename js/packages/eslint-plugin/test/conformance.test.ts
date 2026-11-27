import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { discoverCases, resolveSpecRoot } from "@fldlint/spec";
import { fix, lint, noFixOf, skipOf, type ParserName } from "./helpers.js";

const root = resolveSpecRoot();

interface Expected {
  diagnostics: { line: number; col: number; ruleId: string; skip?: string; noFix?: string }[];
  output: boolean;
}

const sortKey = (d: { line: number; col: number; ruleId: string }) => [d.line, d.col, d.ruleId] as const;
const cmp = (a: { line: number; col: number; ruleId: string }, b: typeof a) =>
  sortKey(a)[0] - sortKey(b)[0] || sortKey(a)[1] - sortKey(b)[1] || a.ruleId.localeCompare(b.ruleId);

describe(`eslint-plugin fixture conformance (spec root: ${root})`, () => {
  for (const c of discoverCases(root)) {
    const expected = (c.data as { expect?: { js?: Expected } } | undefined)?.expect?.js;
    if (!expected) continue;
    const dir = join(c.dir, "js");
    const input = readFileSync(join(dir, "input.ts"), "utf8");
    const output = expected.output ? readFileSync(join(dir, "output.ts"), "utf8") : undefined;

    for (const parser of ["typescript-eslint", "espree"] as ParserName[]) {
      // espree cannot parse TypeScript syntax; run it wherever the fixture is plain JS.
      if (parser === "espree" && lint(input, "espree").some((m) => m.fatal)) continue;

      describe(`${c.relPath} [${parser}]`, () => {
        it("reports the expected diagnostics", () => {
          const msgs = lint(input, parser);
          expect(msgs.filter((m) => m.fatal)).toEqual([]);
          const got = msgs.map((m) => ({
            line: m.line,
            col: m.column,
            ruleId: (m.ruleId ?? "").replace(/^fldlint\//, ""),
            ...(skipOf(m) !== undefined && { skip: skipOf(m) }),
            ...(noFixOf(m) !== undefined && { noFix: noFixOf(m) }),
          }));
          expect([...got].sort(cmp)).toEqual(expected.diagnostics.map((d) => ({ ...d })).sort(cmp));
          // Every finding is a warning under the recommended preset.
          expect(new Set(msgs.map((m) => m.severity))).toEqual(new Set(msgs.length ? [1] : []));
          // Skips and no-fix findings carry no fix; only provable rewrites do.
          for (const m of msgs) expect(m.fix !== undefined).toBe(skipOf(m) === undefined && noFixOf(m) === undefined);
        });

        if (output !== undefined) {
          it("--fix produces the expected output and is idempotent", () => {
            const fixed = fix(input, parser);
            expect(fixed.output).toBe(output);
            expect(fixed.messages).toEqual([]);
            expect(fix(output, parser)).toMatchObject({ fixed: false, output });
            expect(lint(output, parser)).toEqual([]);
          });
        } else {
          it("leaves the source unchanged", () => {
            expect(existsSync(join(dir, "output.ts"))).toBe(false);
            expect(fix(input, parser)).toMatchObject({ fixed: false, output: input });
          });
        }
      });
    }
  }
});
