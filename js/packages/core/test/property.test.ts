// Property tests over generated programs (issue #21, part 1). A fixed seed
// range keeps the run fast and deterministic; a failure prints the seed and the
// source, so `generate(<seed>)` reproduces it.
import { parseForESLint } from "@typescript-eslint/parser";
import { describe, expect, it } from "vitest";
import { analyze, fingerprint, normalizePath, rewrite } from "../src/index.js";
import { generate, generatePath, mulberry32 } from "./gen.js";

const SEEDS = Array.from({ length: 400 }, (_, i) => i + 1);

function parses(code: string): void {
  parseForESLint(code, { sourceType: "module", range: true, loc: true, comment: true, filePath: "input.ts" });
}

/** Independent oracle: the 1-based column counted explicitly in UTF-16 code units (not String.length). */
function utf16Col(prefix: string): number {
  let units = 0;
  for (const ch of prefix) units += (ch.codePointAt(0) as number) > 0xffff ? 2 : 1;
  return units + 1;
}

/** Throw with the seed and program attached, so a failure is reproducible. */
function fail(seed: number, source: string, what: string): never {
  throw new Error(`seed ${seed}: ${what}\n--- source ---\n${source}`);
}

describe("generator", () => {
  it("is deterministic per seed and varies across seeds", () => {
    expect(generate(7).source).toBe(generate(7).source);
    expect(new Set(SEEDS.map((s) => generate(s).source)).size).toBeGreaterThan(SEEDS.length / 2);
    const a = mulberry32(1);
    const b = mulberry32(1);
    expect([a(), a(), a()]).toEqual([b(), b(), b()]);
  });

  it("generates same-line Unicode prefixes whose calls are found at UTF-16 columns", () => {
    let found = 0;
    for (const seed of SEEDS) {
      const g = generate(seed);
      if (!g.choices.uniPrefix) continue;
      const lines = g.source.split("\n");
      const idx = lines.findIndex((l) => l.includes("label = "));
      expect(idx, `seed ${seed}: prefix line missing`).toBeGreaterThan(-1);
      const line = lines[idx] as string;
      expect(line).toContain("caf\u00e9 \u{1F600}");
      const prefix = line.slice(0, line.indexOf("v = "));
      for (const f of analyze(g.source).filter((x) => x.site.line === idx + 1)) {
        // The call starts after the prefix: UTF-16 units exceed code points, which exceed nothing less.
        expect(f.site.col).toBe(utf16Col(line.slice(0, f.site.range[0] - g.source.split("\n").slice(0, idx).join("\n").length - 1)));
        expect(f.site.col).toBeGreaterThan([...prefix].length + 1);
        found++;
      }
    }
    expect(found).toBeGreaterThan(5);
  });

  it("covers every outcome the planner can produce", () => {
    const seen = new Set<string>();
    for (const seed of SEEDS) {
      for (const f of analyze(generate(seed).source)) seen.add(f.result.kind === "skip" ? f.result.category : f.result.kind);
    }
    for (const want of ["rewrite", "dynamic-key", "context-unproven", "default-type-unproven", "unbound-client", "type-mismatch", "detail", "bulk"]) {
      expect([...seen]).toContain(want);
    }
  });
});

describe("planner properties over generated programs", () => {
  it.each(SEEDS)("seed %i", (seed) => {
    const { source } = generate(seed);
    try {
      parses(source);
    } catch (e) {
      return fail(seed, source, `generated source does not parse (generator bug): ${String(e)}`);
    }

    const findings = analyze(source); // no throw
    const out = rewrite(source);

    // The output parses.
    try {
      parses(out);
    } catch (e) {
      fail(seed, `${source}\n--- output ---\n${out}`, `rewritten file does not parse: ${String(e)}`);
    }

    // Idempotent: nothing is left to rewrite, and a second pass changes nothing.
    if (analyze(out).some((f) => f.result.kind === "rewrite")) fail(seed, `${source}\n--- output ---\n${out}`, "output still has a rewritable call");
    if (rewrite(out) !== out) fail(seed, `${source}\n--- output ---\n${out}`, "rewriting is not idempotent");

    // Every skipped (or unfixable) call is byte-identical in the output; with no
    // rewrite at all the whole file is.
    const rewrites = findings.filter((f) => f.result.kind === "rewrite");
    if (rewrites.length === 0 && out !== source) fail(seed, `${source}\n--- output ---\n${out}`, "only skips, but the file changed");
    for (const f of findings) {
      if (f.result.kind === "rewrite") continue;
      const text = source.slice(f.site.range[0], f.site.range[1]);
      if (!out.includes(text)) fail(seed, `${source}\n--- output ---\n${out}`, `skipped call changed: ${JSON.stringify(text)}`);
    }

    // Positions: line/col locate the call start, col counted in UTF-16 units.
    for (const f of findings) {
      const before = source.slice(0, f.site.range[0]);
      const lines = before.split("\n");
      if (f.site.line !== lines.length || f.site.col !== utf16Col(lines.at(-1) as string)) {
        fail(seed, source, `position ${f.site.line}:${f.site.col} does not match range start ${f.site.range[0]}`);
      }
    }
  });
});

describe("path and fingerprint properties over generated paths", () => {
  it.each(SEEDS)("seed %i", (seed) => {
    const p = generatePath(seed);
    const n = normalizePath(p);
    const ctx = `seed ${seed}: ${JSON.stringify(p)} -> ${JSON.stringify(n)}`;
    expect(normalizePath(n), `${ctx}: not idempotent`).toBe(n);
    expect(n, ctx).not.toMatch(/\\|\/\/|\/\.\/|\/$/);
    expect(n.startsWith("./"), ctx).toBe(false);
    const input = { ruleId: "bool-variation", path: p, key: "flag", occ: seed % 3 };
    expect(fingerprint({ ...input, path: n }), `${ctx}: fingerprint changed under re-normalization`).toBe(fingerprint(input));
  });
});
