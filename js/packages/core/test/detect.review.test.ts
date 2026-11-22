import { Linter } from "eslint";
import * as tsParser from "@typescript-eslint/parser";
import { describe, expect, it } from "vitest";
import { analyze, applyEdits, detect, rewrite, type CallSite } from "../src/index.js";

const LD = `import { init } from "@launchdarkly/node-server-sdk";\n`;
const OF = `import { OpenFeature } from "@openfeature/server-sdk";\n`;
const head = `${LD}${OF}const ld = init("x");\n`;
const CALL = `ld.boolVariation("k", { key: u }, false)`;

function outcomes(src: string): string[] {
  return analyze(src).map((f) => (f.result.kind === "rewrite" ? "rewrite" : f.result.category));
}

describe("client binding: declaration position and kind", () => {
  const cases: { name: string; src: string; want: string[] }[] = [
    {
      name: "client is not bound inside its own initializer",
      src: `${head}const c = OpenFeature.getClient(await ld.stringVariation("d", { key: u }, "x"));`,
      want: ["unbound-client"],
    },
    {
      name: "client is not bound inside its own initializer (let, later declarator)",
      src: `${head}let a = 1, c = OpenFeature.getClient(${CALL});`,
      want: ["unbound-client"],
    },
    {
      name: "var in a nested block is rejected",
      src: `${head}if (cond) { var c = OpenFeature.getClient(); }\n${CALL};`,
      want: ["unbound-client"],
    },
    {
      name: "var at the same level is rejected",
      src: `${head}var c = OpenFeature.getClient(); ${CALL};`,
      want: ["unbound-client"],
    },
    {
      name: "let is accepted",
      src: `${head}let c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "block-scoped client does not leak out of its block",
      src: `${head}if (cond) { const c = OpenFeature.getClient(); }\n${CALL};`,
      want: ["unbound-client"],
    },
    {
      name: "call in a different SwitchCase is rejected",
      src: `${head}switch (x) { case 1: const c = OpenFeature.getClient(); break; case 2: ${CALL}; }`,
      want: ["unbound-client"],
    },
    {
      name: "call in the same SwitchCase is accepted",
      src: `${head}switch (x) { case 1: const c = OpenFeature.getClient(); ${CALL}; break; case 2: break; }`,
      want: ["rewrite"],
    },
    {
      name: "inner var does not fall through to an outer const",
      src: `${head}const c = OpenFeature.getClient(); function f() { if (y) { var c = other(); } return ${CALL}; }`,
      want: ["unbound-client"],
    },
  ];
  for (const { name, src, want } of cases) it(name, () => expect(outcomes(src)).toEqual(want));
});

describe("LD receiver binding: declared before use", () => {
  const cases: { name: string; src: string; want: string[] }[] = [
    { name: "receiver in its TDZ", src: `${head.replace('const ld = init("x");\n', "")}const v = ${CALL}; const ld = init("x");`, want: [] },
    { name: "receiver used inside its own initializer", src: `${LD}const ld = init(${CALL});`, want: [] },
    { name: "var receiver", src: `${LD}${OF}const c = OpenFeature.getClient(); var ld = init("x"); ${CALL};`, want: [] },
    { name: "var receiver in a nested block", src: `${LD}${OF}const c = OpenFeature.getClient(); if (y) { var ld = init("x"); }\n${CALL};`, want: [] },
    { name: "receiver in another block", src: `${LD}${OF}const c = OpenFeature.getClient(); if (y) { const ld = init("x"); }\n${CALL};`, want: [] },
    {
      name: "receiver in a different SwitchCase",
      src: `${LD}${OF}const c = OpenFeature.getClient(); switch (x) { case 1: const ld = init("x"); break; case 2: ${CALL}; }`,
      want: [],
    },
    { name: "let receiver declared before use", src: `${LD}${OF}const c = OpenFeature.getClient(); let ld = init("x"); ${CALL};`, want: ["rewrite"] },
  ];
  for (const { name, src, want } of cases) it(name, () => expect(outcomes(src)).toEqual(want));
});

describe("spread and arity apply to report-only rules too", () => {
  const cases: { call: string; want: string[] }[] = [
    { call: `ld.variation(...args)`, want: [] },
    { call: `ld.variation("k", ...rest)`, want: [] },
    { call: `ld.allFlagsState(...rest)`, want: [] },
    { call: `ld.boolVariationDetail("k", ctx, d, ...x)`, want: [] },
    { call: `ld.variation("k", ctx, d)`, want: ["type-mismatch"] },
    { call: `ld.allFlagsState(ctx)`, want: ["bulk"] },
  ];
  for (const { call, want } of cases) it(call, () => expect(outcomes(`${head}${call};`)).toEqual(want));
});

describe("receiver proofs ignore type-only imports", () => {
  const cases: { name: string; src: string; want: string[] }[] = [
    {
      name: "import type { init }",
      src: `import type { init } from "@launchdarkly/node-server-sdk";${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: [],
    },
    {
      name: "import { type init }",
      src: `import { type init } from "@launchdarkly/node-server-sdk";${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: [],
    },
    {
      name: "import type * as sdk",
      src: `import type * as sdk from "@launchdarkly/node-server-sdk";${OF}const ld = sdk.init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: [],
    },
    {
      name: "type-only OpenFeature import does not bind a client",
      src: `${LD}import type { OpenFeature } from "@openfeature/server-sdk";\nconst ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["unbound-client"],
    },
    {
      name: "import { type X, init } still proves init",
      src: `import { type Foo, init } from "@launchdarkly/node-server-sdk";${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
  ];
  for (const { name, src, want } of cases) it(name, () => expect(outcomes(src)).toEqual(want));
});

describe("default literals must be JSON", () => {
  const body = (d: string) => `${head}const c = OpenFeature.getClient(); ld.jsonVariation("k", { key: u }, ${d});`;
  const cases: { d: string; want: string }[] = [
    { d: `{ a: [1, "x", null, true, -2], "b-c": { d: \`t\` } }`, want: "rewrite" },
    { d: `[]`, want: "rewrite" },
    { d: `{ ...x }`, want: "default-type-unproven" },
    { d: `[...x]`, want: "default-type-unproven" },
    { d: `{ [k]: 1 }`, want: "default-type-unproven" },
    { d: `{ a: sideEffect() }`, want: "default-type-unproven" },
    { d: `{ a }`, want: "default-type-unproven" },
    { d: `{ a: () => 1 }`, want: "default-type-unproven" },
    { d: `{ a() {} }`, want: "default-type-unproven" },
    { d: `{ get a() { return 1; } }`, want: "default-type-unproven" },
    { d: `{ a: new Date() }`, want: "default-type-unproven" },
    { d: `[1, , 2]`, want: "default-type-unproven" },
    { d: `{ a: /re/ }`, want: "default-type-unproven" },
    { d: `{ a: 1n }`, want: "default-type-unproven" },
    { d: "{ a: `x${y}` }", want: "default-type-unproven" },
    { d: `{ __proto__: null }`, want: "default-type-unproven" },
    { d: `{ a: [ { b: f() } ] }`, want: "default-type-unproven" },
  ];
  for (const { d, want } of cases) it(d, () => expect(outcomes(body(d))).toEqual([want]));
});

describe("BOM", () => {
  const one = `const ld = require("@launchdarkly/node-server-sdk").init("x"); const c = require("@openfeature/server-sdk").OpenFeature.getClient(); ${CALL};`;
  const bom = `\uFEFF${one}`;
  it("line-1 columns match the BOM-stripped text (as ESLint sees it)", () => {
    expect(analyze(bom)[0]?.site.col).toBe(one.indexOf(CALL) + 1);
  });
  it("ranges are in original-source coordinates and usable with applyEdits", () => {
    const [f] = analyze(bom);
    expect(f?.site.range).toEqual([bom.indexOf(CALL), bom.indexOf(CALL) + CALL.length]);
    expect(bom.slice(...f!.site.range)).toBe(CALL);
    if (f?.result.kind !== "rewrite") throw new Error("expected rewrite");
    expect(applyEdits(bom, [f.result.edit])).toBe(`\uFEFF${one.replace(CALL, `c.getBooleanValue("k", false, { targetingKey: u })`)}`);
  });
  it("analyze() line/col equal what ESLint's own (BOM-stripped) text gives detect()", () => {
    const cols: number[] = [];
    new Linter().verify(
      bom,
      [
        {
          languageOptions: { parser: tsParser, sourceType: "commonjs" },
          plugins: {
            t: {
              rules: {
                collect: {
                  create(ctx: { sourceCode: { ast: never; scopeManager: never; text: string; visitorKeys: never } }) {
                    const sc = ctx.sourceCode;
                    return { "Program:exit"() { cols.push(...detect(sc.ast, sc.scopeManager, sc.text, sc.visitorKeys).map((x) => x.col)); } };
                  },
                },
              },
            },
          },
          rules: { "t/collect": "error" },
        },
      ] as never,
      "file.js",
    );
    expect(cols).toEqual(analyze(bom).map((f) => f.site.col));
    expect(cols).toHaveLength(1);
  });
  it("rewrite keeps the BOM and edits the right range", () => {
    expect(rewrite(bom)).toBe(`\uFEFF${one.replace(CALL, `c.getBooleanValue("k", false, { targetingKey: u })`)}`);
  });
});

describe("CommonJS require under ESLint", () => {
  const code = `const { init } = require("@launchdarkly/node-server-sdk");
const { OpenFeature } = require("@openfeature/server-sdk");
const ld = init("x"); const c = OpenFeature.getClient();
${CALL};`;

  function run(languageOptions: Record<string, unknown>): CallSite[] {
    const sites: CallSite[] = [];
    const linter = new Linter();
    const messages = linter.verify(
      code,
      [
        {
          files: ["**/*.js"],
          languageOptions: { parser: tsParser, sourceType: "commonjs", ...languageOptions },
          plugins: {
            t: {
              rules: {
                collect: {
                  create(ctx: { sourceCode: { ast: never; scopeManager: never; text: string; visitorKeys: never } }) {
                    const sc = ctx.sourceCode;
                    return { "Program:exit"() { sites.push(...detect(sc.ast, sc.scopeManager, sc.text, sc.visitorKeys)); } };
                  },
                },
              },
            },
          },
          rules: { "t/collect": "error" },
        },
      ] as never,
      "file.js",
    );
    expect(messages).toEqual([]);
    return sites;
  }

  it("proves require when ESLint has no require global configured", () => {
    expect(run({}).map((s) => s.clientBinding)).toEqual([{ kind: "bound", name: "c" }]);
  });
  it("proves require when globals configures it as a readonly global", () => {
    expect(run({ globals: { require: "readonly" } }).map((s) => s.clientBinding)).toEqual([{ kind: "bound", name: "c" }]);
  });
  it("proves require with globals.node-style config (many globals)", () => {
    expect(run({ globals: { require: "readonly", module: "writable", process: "readonly" } })).toHaveLength(1);
  });
  it("still rejects a locally shadowed require under ESLint", () => {
    const sites: CallSite[] = [];
    const linter = new Linter();
    linter.verify(
      `function g(require) { const { init } = require("@launchdarkly/node-server-sdk"); const ld = init("x"); return ${CALL}; }`,
      [
        {
          languageOptions: { parser: tsParser, sourceType: "commonjs", globals: { require: "readonly" } },
          plugins: {
            t: {
              rules: {
                collect: {
                  create(ctx: { sourceCode: { ast: never; scopeManager: never; text: string; visitorKeys: never } }) {
                    const sc = ctx.sourceCode;
                    return { "Program:exit"() { sites.push(...detect(sc.ast, sc.scopeManager, sc.text, sc.visitorKeys)); } };
                  },
                },
              },
            },
          },
          rules: { "t/collect": "error" },
        },
      ] as never,
      "file.js",
    );
    expect(sites).toEqual([]);
  });
});
