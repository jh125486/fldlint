import { readFileSync } from "node:fs";
import { join } from "node:path";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { resolveSpecRoot } from "@fldlint/spec";
import { analyze, assignOccurrences, rewrite, fingerprint, normalizePath, staticFingerprintKey, type FingerprintInput } from "../src/index.js";

const contracts = join(resolveSpecRoot(), "contracts");
const vectors = JSON.parse(readFileSync(join(contracts, "fingerprint-vectors.json"), "utf8")) as (FingerprintInput & {
  fingerprint: string;
})[];

const illformed = JSON.parse(readFileSync(join(contracts, "fingerprint-illformed-vectors.json"), "utf8")) as {
  ruleId: string;
  path: string;
  keyUtf16: number[];
  occ: number;
  fingerprint: string;
}[];

const LD = `import { init } from "@launchdarkly/node-server-sdk";\nconst ld = init("x");\n`;

/** [ruleId, keyValue] of each finding, with `occ` assigned as a scan would. */
function identities(body: string): { ruleId: string; key: string | null; occ: number }[] {
  const found = analyze(LD + body).map((f) => ({ ruleId: f.site.ruleId, key: f.site.keyValue ?? null, line: f.site.line, col: f.site.col }));
  return assignOccurrences(found).map(({ ruleId, key, occ }) => ({ ruleId, key, occ }));
}

describe("fingerprint", () => {
  it.each(vectors.map((v) => [JSON.stringify(v.path) + " " + JSON.stringify(v.key) + " " + v.occ, v] as const))(
    "vector %s",
    (_n, v) => expect(fingerprint(v)).toBe(v.fingerprint),
  );

  it("has exactly the documented vectors, covering absent/empty keys and several occ values", () => {
    const rows = readFileSync(join(contracts, "fingerprint.md"), "utf8").split("\n").filter((l) => /^\| [a-z-]+ \| `/.test(l));
    expect(vectors).toHaveLength(rows.length);
    expect(vectors.length).toBeGreaterThanOrEqual(25);
    expect(vectors.every((v) => /^[0-9a-f]{32}$/.test(v.fingerprint))).toBe(true);
    expect(vectors.some((v) => v.key === null)).toBe(true);
    expect(vectors.some((v) => v.key === "")).toBe(true);
    expect(vectors.some((v) => v.occ > 0)).toBe(true);
  });

  it("keeps fingerprint-vectors.json and the fingerprint.md table in agreement", () => {
    const md = readFileSync(join(contracts, "fingerprint.md"), "utf8");
    const rows = md
      .split("\n")
      .filter((l) => /^\| [a-z-]+ \| `/.test(l))
      .map((l) => l.slice(1, -1).split(" | ").map((c) => c.trim()));
    const unq = (c: string) => c.replace(/^`|`$/g, "");
    const fromDoc = rows.map(([ruleId, path, key, occ, fp]) => ({
      ruleId: ruleId!,
      path: unq(path!),
      key: key === "(none)" ? null : key === "(empty string)" ? "" : unq(key!),
      occ: Number(occ),
      fingerprint: unq(fp!),
    }));
    expect(fromDoc).toEqual(vectors);
  });

  it("normalized-path vectors equal the fingerprint of the normalized form", () => {
    for (const v of vectors) expect(fingerprint({ ...v, path: normalizePath(v.path) })).toBe(v.fingerprint);
  });

  it("distinguishes absent and empty keys, and NFC from NFD", () => {
    const base = { ruleId: "r", path: "p", occ: 0 };
    expect(fingerprint({ ...base, key: null })).not.toBe(fingerprint({ ...base, key: "" }));
    expect(fingerprint({ ...base, path: "\u00e9", key: null })).not.toBe(fingerprint({ ...base, path: "e\u0301", key: null }));
  });

  it("uses the decoded key value of JS literals", () => {
    const head = `import { init } from "@launchdarkly/node-server-sdk";\nconst ld = init("x");\n`;
    const keys = ["'a\\u0041'", '"a\\x41"', "`aA`", "'aA'"].map((lit) => {
      const [f] = analyze(`${head}ld.boolVariation(${lit}, { key: u }, false);`);
      return f?.site.keyValue;
    });
    expect(keys).toEqual(["aA", "aA", "aA", "aA"]);
  });
});

describe("report-only rules have a key (M0.9)", () => {
  const cases: { name: string; call: (k: string) => string; ruleId: string }[] = [
    { name: "variation", call: (k) => `ld.variation(${k}, { key: u }, false)`, ruleId: "variation" },
    { name: "variationDetail", call: (k) => `ld.variationDetail(${k}, { key: u }, false)`, ruleId: "variation-detail" },
    { name: "boolVariationDetail", call: (k) => `ld.boolVariationDetail(${k}, { key: u }, false)`, ruleId: "bool-variation-detail" },
    { name: "stringVariationDetail", call: (k) => `ld.stringVariationDetail(${k}, { key: u }, "")`, ruleId: "string-variation-detail" },
    { name: "numberVariationDetail", call: (k) => `ld.numberVariationDetail(${k}, { key: u }, 0)`, ruleId: "number-variation-detail" },
    { name: "jsonVariationDetail", call: (k) => `ld.jsonVariationDetail(${k}, { key: u }, {})`, ruleId: "json-variation-detail" },
    { name: "migrationVariation", call: (k) => `ld.migrationVariation(${k}, { key: u }, "off")`, ruleId: "migration-variation" },
  ];

  it.each(cases)("$name: static key extracted, dynamic key and bulk stay keyless", ({ call, ruleId }) => {
    expect(identities(`${call('"flag-a"')};\n${call("`flag-b`")};\n${call("name")};`)).toEqual([
      { ruleId, key: "flag-a", occ: 0 },
      { ruleId, key: "flag-b", occ: 0 },
      { ruleId, key: null, occ: 0 },
    ]);
    expect(identities(`ld.allFlagsState({ key: u });`)).toEqual([{ ruleId: "all-flags-state", key: null, occ: 0 }]);
  });

  it("deleting one call does not change the fingerprint of the other", () => {
    const fp = (body: string) => identities(body).map((i) => fingerprint({ ...i, path: "src/flags.ts" }));
    const both = fp(`ld.variation("flag-a", { key: u }, false);\nld.variation("flag-b", { key: u }, false);`);
    const onlyB = fp(`ld.variation("flag-b", { key: u }, false);`);
    expect(both[0]).not.toBe(both[1]);
    expect(onlyB).toEqual([both[1]]);
  });
});

describe("keys that are not well formed", () => {
  const unit = (units: number[]) => String.fromCharCode(...units);

  it.each(illformed.map((v) => [JSON.stringify(v.keyUtf16), v] as const))("vector %s is treated as absent", (_n, v) => {
    const key = unit(v.keyUtf16);
    expect(key.isWellFormed()).toBe(false);
    expect(staticFingerprintKey(key)).toBeNull();
    expect(fingerprint({ ruleId: v.ruleId, path: v.path, key, occ: v.occ })).toBe(v.fingerprint);
  });

  it("does not collide where U+FFFD hashing would, and never throws", () => {
    const base = { ruleId: "r", path: "p", occ: 0 };
    const a = unit([0x61, 0xd800]);
    const b = unit([0x61, 0xd801]);
    expect(() => fingerprint({ ...base, key: a })).not.toThrow();
    // Both are absent, so they match the absent-key fingerprint, not each other's via U+FFFD.
    expect(fingerprint({ ...base, key: a })).toBe(fingerprint({ ...base, key: null }));
    expect(fingerprint({ ...base, key: b })).toBe(fingerprint({ ...base, key: null }));
    expect(fingerprint({ ...base, key: "a\uFFFD" })).not.toBe(fingerprint({ ...base, key: a }));
  });

  it("gives two ill-formed keys in one file distinct occ", () => {
    const f = (key: string | null, line: number) => ({ ruleId: "r", key, line, col: 1 });
    const got = assignOccurrences([f(unit([0x61, 0xd800]), 1), f(unit([0x61, 0xd801]), 2), f(null, 3)]);
    expect(got.map((x) => x.occ)).toEqual([0, 1, 2]);
  });

  it("leaves keyValue absent on a call site but still rewrites it", () => {
    const src = `import { OpenFeature } from "@openfeature/server-sdk";\n${LD}const of = OpenFeature.getClient();\nld.boolVariation('a\\uD800', { key: u }, false);`;
    const [f] = analyze(src);
    expect(f?.site.keyValue).toBeUndefined();
    expect(f?.site.keyIsStatic).toBe(true);
    expect(f?.result.kind).toBe("rewrite");
    // The edit keeps the original key literal, written as the source spelled it.
    expect(rewrite(src)).toContain("of.getBooleanValue('a\\uD800', false, { targetingKey: u })");
  });
});

describe("normalizePath", () => {
  const cases: [string, string][] = [
    ["a//b", "a/b"],
    ["a/./b", "a/b"],
    ["a/././b", "a/b"],
    ["././a", "a"],
    ["a/b/", "a/b"],
    [".\\a\\b", "a/b"],
    ["src/ünï/a.ts", "src/ünï/a.ts"],
    ["A/b", "A/b"],
  ];
  it.each(cases)("%s -> %s", (input, want) => expect(normalizePath(input)).toBe(want));

  it("is idempotent and never starts with ./ or ends with /", () => {
    fc.assert(
      fc.property(fc.stringMatching(/^[a-z.\\/]{0,20}$/), (p) => {
        const n = normalizePath(p);
        expect(normalizePath(n)).toBe(n);
        expect(n.startsWith("./")).toBe(false);
        expect(n.includes("\\") || n.includes("//") || n.includes("/./") || n.endsWith("/")).toBe(false);
      }),
    );
  });
});

describe("assignOccurrences", () => {
  it("ranks by (line, col) within (ruleId, kind, key) and preserves input order", () => {
    const f = (ruleId: string, key: string | null, line: number, col: number) => ({ ruleId, key, line, col });
    const got = assignOccurrences([
      f("r", "k", 10, 5),
      f("r", "k", 2, 9),
      f("r", "k", 2, 3),
      f("r", null, 1, 1),
      f("r", "", 3, 1),
      f("r", null, 0, 1),
      f("s", "k", 1, 1),
    ]);
    expect(got.map((x) => x.occ)).toEqual([2, 1, 0, 1, 0, 0, 0]);
  });
});
