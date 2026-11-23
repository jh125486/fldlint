import { readFileSync } from "node:fs";
import { join } from "node:path";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { resolveSpecRoot } from "@fldlint/spec";
import { analyze, assignOccurrences, fingerprint, normalizePath, type FingerprintInput } from "../src/index.js";

const contracts = join(resolveSpecRoot(), "contracts");
const vectors = JSON.parse(readFileSync(join(contracts, "fingerprint-vectors.json"), "utf8")) as (FingerprintInput & {
  fingerprint: string;
})[];

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
