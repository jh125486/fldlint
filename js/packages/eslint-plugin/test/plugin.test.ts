import { describe, expect, it } from "vitest";
import { RuleTester } from "eslint";
import { rules as specRules, skipCategories } from "@fldlint/spec";
import plugin, { configs, rules } from "../src/index.js";
import { lint, fix, skipOf } from "./helpers.js";

describe("plugin shape", () => {
  it("has one rule per spec rule id that has JS methods", () => {
    const expected = specRules.filter((r) => r.languages.js).map((r) => r.id).sort();
    expect(Object.keys(rules).sort()).toEqual(expected);
    expect(plugin.rules).toBe(rules);
  });

  it("declares meta with namespace fldlint", () => {
    expect(plugin.meta).toMatchObject({ name: "eslint-plugin-fldlint", namespace: "fldlint" });
  });

  it("recommended sets every rule to warn and references the plugin", () => {
    expect(configs.recommended.plugins?.fldlint).toBe(plugin);
    const entries = Object.entries(configs.recommended.rules ?? {});
    expect(entries.map(([k]) => k).sort()).toEqual(Object.keys(rules).map((id) => `fldlint/${id}`).sort());
    expect(new Set(entries.map(([, v]) => v))).toEqual(new Set(["warn"]));
    expect((plugin.configs as Record<string, unknown>).recommended).toBe(configs.recommended);
    expect(configs.recommended.languageOptions).toBeUndefined();
  });

  it("every rule has docs.url, fixable code, an empty schema and spec-derived messages", () => {
    for (const [id, rule] of Object.entries(rules)) {
      const spec = specRules.find((r) => r.id === id)!;
      expect(rule.meta?.type).toBe("problem");
      expect(rule.meta?.fixable).toBe("code");
      expect(rule.meta?.schema).toEqual([]);
      expect(rule.meta?.docs?.url).toMatch(/#.+/);
      expect(rule.meta?.docs?.description).toBeTruthy();
      expect(rule.meta?.messages?.migrate).toBe(spec.message);
      for (const [cat, { message }] of Object.entries(skipCategories)) {
        expect(rule.meta?.messages?.[`skip-${cat}`]).toBe(`${message} [skip: ${cat}]`);
      }
    }
  });
});

const ts = `import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";
const ld = init("k");
const client = OpenFeature.getClient();
export const a = ld.boolVariation("a", { key: "u" }, false);
export const b = ld.boolVariation(name, { key: "u" }, false);
export const c = ld.allFlagsState({ key: "u" });
`;

describe("messages", () => {
  it("substitutes the key and appends the skip category", () => {
    const m = lint(ts, "espree");
    expect(m.map((x) => x.message)).toEqual([
      'Migrate boolean flag "a" to OpenFeature',
      "Flag key is not a static string [skip: dynamic-key]",
      "Bulk evaluation; migrate manually [skip: bulk]",
    ]);
    expect(m.map(skipOf)).toEqual([undefined, "dynamic-key", "bulk"]);
  });

  it("reports the end of the call range", () => {
    const m = lint(ts, "espree")[0]!;
    const call = 'ld.boolVariation("a", { key: "u" }, false)';
    expect(m).toMatchObject({ line: 5, column: 18, endLine: 5, endColumn: 18 + call.length });
  });

  it("pins end line and column for skips and multi-line calls", () => {
    const msgs = lint(ts, "espree");
    const bulk = msgs[2]!;
    const call = "ld.allFlagsState({ key: \"u\" })";
    expect(bulk).toMatchObject({ line: 7, column: 18, endLine: 7, endColumn: 18 + call.length });
    const multi = ts.replace('ld.boolVariation("a", { key: "u" }, false)', 'ld.boolVariation(\n  "a",\n  { key: "u" },\n  false)');
    expect(lint(multi, "espree")[0]).toMatchObject({ line: 5, column: 18, endLine: 8, endColumn: 9 });
  });
});

describe("positions", () => {
  it("converts to 1-based UTF-16 columns after non-ascii text", () => {
    const src = ts.replace(
      'export const a = ld.',
      'export const a = "café 😀" + ld.',
    );
    const m = lint(src, "espree")[0]!;
    // "export const a = " (17) + `"café 😀"` (8 chars, one astral = 2 units: 9) + " + " (3) = 29, so col 30.
    expect(m.column).toBe(30);
  });
});

describe("severity configuration", () => {
  it("honours off, warn and error per rule", () => {
    const m = lint(ts, "espree", { "fldlint/bool-variation": "error", "fldlint/all-flags-state": "off" });
    expect(m.map((x) => [x.ruleId, x.severity])).toEqual([["fldlint/bool-variation", 2], ["fldlint/bool-variation", 2]]);
  });

  it("disables inline with eslint-disable-next-line fldlint/<rule>", () => {
    const src = ts.replace("export const a", "// eslint-disable-next-line fldlint/bool-variation\nexport const a");
    expect(lint(src, "espree").map((x) => x.line)).toEqual([7, 8]);
  });
});

describe("comments inside a call (issue #31)", () => {
  const prelude = `import { init } from "@launchdarkly/node-server-sdk";\nimport { OpenFeature } from "@openfeature/server-sdk";\nconst ld = init("x");\nconst c = OpenFeature.getClient();\n`;
  for (const parser of ["typescript-eslint", "espree"] as const) {
    it(`${parser}: a comment moves with its argument under --fix`, () => {
      const src = `${prelude}const v = ld.boolVariation("k", /* who */ { key: u }, false /* off */);\n`;
      expect(fix(src, parser).output).toBe(`${prelude}const v = c.getBooleanValue("k", false /* off */, /* who */ { targetingKey: u });\n`);
    });
    it(`${parser}: a comment with no home is reported without a fix`, () => {
      const src = `${prelude}const v = ld.boolVariation("k", { key: u /* id */ }, false);\n`;
      const msgs = lint(src, parser);
      expect(msgs).toHaveLength(1);
      expect(msgs[0]?.message).toBe('Migrate boolean flag "k" to OpenFeature [no-fix: comment]');
      expect(msgs[0]?.fix).toBeUndefined();
      expect(skipOf(msgs[0]!)).toBeUndefined();
      expect(fix(src, parser).output).toBe(src);
    });
  }
});

describe("legacy package", () => {
  it("never rewrites launchdarkly-node-server-sdk calls", () => {
    const src = ts.replace("@launchdarkly/node-server-sdk", "launchdarkly-node-server-sdk");
    expect(fix(src, "espree").output).toBe(src);
  });
});

const tester = new RuleTester({ languageOptions: { ecmaVersion: "latest", sourceType: "module" } });
RuleTester.describe = describe as never;
RuleTester.it = it as never;
RuleTester.itOnly = it.only as never;
tester.run("fldlint/bool-variation", rules["bool-variation"]!, {
  valid: [
    `import { init } from "@launchdarkly/node-server-sdk";\nconst ld = other();\nld.boolVariation("a", { key: "u" }, false);`,
    `const x = { boolVariation() {} }; x.boolVariation("a", { key: "u" }, false);`,
  ],
  invalid: [
    {
      code: `import { init } from "@launchdarkly/node-server-sdk";\nimport { OpenFeature } from "@openfeature/server-sdk";\nconst ld = init("k");\nconst client = OpenFeature.getClient();\nld.boolVariation("a", { key: "u" }, false);`,
      errors: [{ messageId: "migrate", data: { key: '"a"' }, line: 5, column: 1 }],
      output: `import { init } from "@launchdarkly/node-server-sdk";\nimport { OpenFeature } from "@openfeature/server-sdk";\nconst ld = init("k");\nconst client = OpenFeature.getClient();\nclient.getBooleanValue("a", false, { targetingKey: "u" });`,
    },
    {
      code: `import { init } from "@launchdarkly/node-server-sdk";\nconst ld = init("k");\nld.boolVariation("a", { key: "u" }, false);`,
      errors: [{ messageId: "skip-unbound-client" }],
      output: null,
    },
  ],
});

describe("report-only rules", () => {
  const head = `import { init } from "@launchdarkly/node-server-sdk";
const ld = init("k");
`;
  const cases: [string, string, string][] = [
    ["variation", 'ld.variation("a", { key: "u" }, false)', "type-mismatch"],
    ["migration-variation", 'ld.migrationVariation("a", { key: "u" }, "off")', "type-mismatch"],
    ["bool-variation-detail", 'ld.boolVariationDetail("a", { key: "u" }, false)', "detail"],
    ["string-variation-detail", 'ld.stringVariationDetail("a", { key: "u" }, "")', "detail"],
    ["number-variation-detail", 'ld.numberVariationDetail("a", { key: "u" }, 0)', "detail"],
    ["json-variation-detail", 'ld.jsonVariationDetail("a", { key: "u" }, {})', "detail"],
    ["variation-detail", 'ld.variationDetail("a", { key: "u" }, 0)', "detail"],
    ["all-flags-state", 'ld.allFlagsState({ key: "u" })', "bulk"],
  ];
  for (const [id, call, category] of cases) {
    it(`${id} reports [skip: ${category}] without a fix, only under its own rule`, () => {
      const src = `${head}export const x = ${call};\n`;
      const msgs = lint(src, "espree");
      expect(msgs).toHaveLength(1);
      expect(msgs[0]).toMatchObject({
        ruleId: `fldlint/${id}`,
        severity: 1,
        line: 3,
        column: 18,
        endLine: 3,
        endColumn: 18 + call.length,
      });
      expect(skipOf(msgs[0]!)).toBe(category);
      expect(msgs[0]!.fix).toBeUndefined();
      // Other rules stay quiet when only this one is enabled and vice versa.
      expect(lint(src, "espree", { [`fldlint/${id}`]: "off" })).toEqual([]);
      const others = Object.fromEntries(Object.keys(rules).filter((r) => r !== id).map((r) => [`fldlint/${r}`, "warn" as const]));
      expect(lint(src, "espree", others)).toEqual([]);
    });
  }

  it("typed rewritable rules report their own ids", () => {
    for (const [id, call] of [
      ["string-variation", 'ld.stringVariation("a", { key: "u" }, "")'],
      ["number-variation", 'ld.numberVariation("a", { key: "u" }, 0)'],
      ["json-variation", 'ld.jsonVariation("a", { key: "u" }, {})'],
    ] as const) {
      const msgs = lint(`${head}export const x = ${call};\n`, "espree");
      expect(msgs.map((m) => m.ruleId)).toEqual([`fldlint/${id}`]);
      expect(skipOf(msgs[0]!)).toBe("unbound-client");
    }
  });
});
