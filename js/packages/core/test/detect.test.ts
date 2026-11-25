import { describe, expect, it } from "vitest";
import { analyze, rewrite } from "../src/index.js";

const LD = `import { init } from "@launchdarkly/node-server-sdk";\n`;
const OF = `import { OpenFeature } from "@openfeature/server-sdk";\n`;
const CALL = `ld.boolVariation("k", { key: u }, false)`;

function outcomes(src: string): string[] {
  return analyze(src).map((f) => (f.result.kind === "skip" ? f.result.category : f.result.kind));
}

describe("LD receiver proof (no finding unless proven)", () => {
  const cases: { name: string; src: string; want: string[] }[] = [
    { name: "named import", src: `${LD}${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`, want: ["rewrite"] },
    {
      name: "aliased import",
      src: `import { init as start } from "@launchdarkly/node-server-sdk";${OF}const ld = start("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "namespace import",
      src: `import * as sdk from "@launchdarkly/node-server-sdk";${OF}const ld = sdk.init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "default import namespace",
      src: `import sdk from "@launchdarkly/node-server-sdk";${OF}const ld = sdk.init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "require namespace",
      src: `const sdk = require("@launchdarkly/node-server-sdk");${OF}const ld = sdk.init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "destructured require with rename",
      src: `const { init: start } = require("@launchdarkly/node-server-sdk");${OF}const ld = start("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "inline require(...).init",
      src: `${OF}const ld = require("@launchdarkly/node-server-sdk").init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "legacy package: typed call is not a finding",
      src: `import { init } from "launchdarkly-node-server-sdk";${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: [],
    },
    {
      name: "legacy package: variation keeps its rule-level skip",
      src: `import { init } from "launchdarkly-node-server-sdk";${OF}const ld = init("x"); const c = OpenFeature.getClient(); ld.variation("k", { key: u }, false);`,
      want: ["type-mismatch"],
    },
    {
      name: "new package still rewrites",
      src: `import { init } from "@launchdarkly/node-server-sdk";${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "legacy via require is also never rewritten",
      src: `const { init } = require("launchdarkly-node-server-sdk");${OF}const ld = init("x"); const c = OpenFeature.getClient(); ${CALL};`,
      want: [],
    },
    { name: "init from another package", src: `import { init } from "other";${OF}const ld = init("x"); ${CALL};`, want: [] },
    { name: "init with no import", src: `const ld = init("x"); ${CALL};`, want: [] },
    { name: "method name alone is not enough", src: `declare const ld: any; ${CALL};`, want: [] },
    { name: "parameter receiver", src: `${LD}export function f(ld: any) { return ${CALL}; }`, want: [] },
    {
      name: "shadowed receiver name",
      src: `${LD}${OF}const ld = init("x"); const c = OpenFeature.getClient(); export function f(ld: any) { return ${CALL}; }`,
      want: [],
    },
    { name: "reassigned let", src: `${LD}let ld = init("x"); ld = other(); ${CALL};`, want: [] },
    {
      name: "shadowed require",
      src: `function g(require: any) { const { init } = require("@launchdarkly/node-server-sdk"); const ld = init("x"); return ${CALL}; }`,
      want: [],
    },
    { name: "optional call", src: `${LD}const ld = init("x"); ld.boolVariation?.("k", { key: u }, false);`, want: [] },
    { name: "computed member", src: `${LD}const ld = init("x"); ld["boolVariation"]("k", { key: u }, false);`, want: [] },
    { name: "member receiver", src: `${LD}const o = { ld: init("x") }; o.ld.boolVariation("k", { key: u }, false);`, want: [] },
  ];
  for (const { name, src, want } of cases) {
    it(name, () => expect(outcomes(src)).toEqual(want));
  }
});

describe("OpenFeature client binding proof", () => {
  const head = `${LD}${OF}const ld = init("x");\n`;
  const cases: { name: string; src: string; want: string[] }[] = [
    {
      name: "aliased OpenFeature export binds the alias",
      src: `${LD}import { OpenFeature as OF } from "@openfeature/server-sdk";\nconst ld = init("x"); const client = OF.getClient(); ${CALL};`,
      want: ["rewrite"],
    },
    {
      name: "a local named OpenFeature that is not the export does not bind",
      src: `${LD}const OpenFeature = { getClient: () => 1 }; const ld = init("x"); const client = OpenFeature.getClient(); ${CALL};`,
      want: ["unbound-client"],
    },
    { name: "no client at all", src: `${LD}const ld = init("x"); ${CALL};`, want: ["unbound-client"] },
    {
      name: "typed declarator and multi-declarator",
      src: `${head}const n = 1, client: Client = OpenFeature.getClient(), m = 2; ${CALL};`,
      want: ["rewrite"],
    },
    { name: "two candidates in one scope", src: `${head}const a = OpenFeature.getClient(); const b = OpenFeature.getClient(); ${CALL};`, want: ["unbound-client"] },
    {
      name: "inner candidate wins over outer",
      src: `${head}const a = OpenFeature.getClient(); export function f() { const b = OpenFeature.getClient(); return ${CALL}; }`,
      want: ["rewrite"],
    },
    {
      name: "candidate in non-enclosing function is never picked",
      src: `${head}function f() { const billing = OpenFeature.getClient(); return billing; }\n${CALL};`,
      want: ["unbound-client"],
    },
    {
      name: "non-candidate shadow",
      src: `${head}const client = OpenFeature.getClient(); export function f(client: any) { return ${CALL}; }`,
      want: ["unbound-client"],
    },
    {
      name: "candidate declared after the call",
      src: `${head}${CALL};\nconst client = OpenFeature.getClient();`,
      want: ["unbound-client"],
    },
    {
      name: "reassigned let is not a candidate",
      src: `${head}let client = OpenFeature.getClient(); client = other(); ${CALL};`,
      want: ["unbound-client"],
    },
  ];
  for (const { name, src, want } of cases) {
    it(name, () => expect(outcomes(src)).toEqual(want));
  }

  it("uses the alias as {{client}}", () => {
    const src = `${LD}import { OpenFeature as OF } from "@openfeature/server-sdk";\nconst ld = init("x"); const cl = OF.getClient(); ${CALL};`;
    expect(rewrite(src)).toContain(`cl.getBooleanValue("k", false, { targetingKey: u })`);
  });
});

describe("planner steps and order", () => {
  const head = `${LD}${OF}const ld = init("x"); const client = OpenFeature.getClient();\n`;
  const cases: { name: string; call: string; want: string }[] = [
    { name: "dynamic key", call: `ld.boolVariation(k, { key: u }, false)`, want: "dynamic-key" },
    { name: "template with expression", call: "ld.boolVariation(`a-${x}`, { key: u }, false)", want: "dynamic-key" },

    { name: "identifier context", call: `ld.boolVariation("k", ctx, false)`, want: "context-unproven" },
    { name: "spread in context", call: `ld.boolVariation("k", { ...c }, false)`, want: "context-unproven" },
    { name: "computed context key", call: `ld.boolVariation("k", { ["key"]: u }, false)`, want: "context-unproven" },
    { name: "other kind", call: `ld.boolVariation("k", { kind: "org", key: u }, false)`, want: "context-unproven" },
    { name: "duplicate key", call: `ld.boolVariation("k", { key: u, key: v }, false)`, want: "context-unproven" },
    { name: "kind without key", call: `ld.boolVariation("k", { kind: "user" }, false)`, want: "context-unproven" },
    { name: "sequence-expression key value", call: `ld.boolVariation("k", { key: (a, b) }, false)`, want: "context-unproven" },
    { name: "dynamic key wins over context", call: `ld.boolVariation(k, ctx, d)`, want: "dynamic-key" },
    { name: "context wins over default", call: `ld.boolVariation("k", ctx, d)`, want: "context-unproven" },
    { name: "default wins over client", call: `ld.boolVariation("k", { key: u }, d)`, want: "default-type-unproven" },
    { name: "string default for boolean rule", call: `ld.boolVariation("k", { key: u }, "yes")`, want: "default-type-unproven" },
    { name: "rule-level detail", call: `ld.boolVariationDetail(k, ctx, d)`, want: "detail" },
    { name: "rule-level bulk", call: `ld.allFlagsState(ctx)`, want: "bulk" },
    { name: "rule-level type-mismatch", call: `ld.variation("k", ctx, d)`, want: "type-mismatch" },
  ];
  for (const { name, call, want } of cases) {
    it(name, () => expect(outcomes(`${head}${call};`)).toEqual([want]));
  }

  it("calls that do not match the SDK arity are unrecognized: no finding, source unchanged", () => {
    for (const call of [
      `ld.boolVariation("k", { key: u }, false, sideEffect())`,
      `ld.boolVariation("k", { key: u }, false, ...rest)`,
      `ld.boolVariation(...args)`,
      `ld.boolVariation("k", ...rest)`,
      `ld.boolVariation("k", { key: u })`,
      `ld.boolVariation()`,
    ]) {
      const src = `${head}${call};`;
      expect(analyze(src), call).toEqual([]);
      expect(rewrite(src), call).toBe(src);
    }
  });

  it("default-type-unproven wins over a genuinely unbound client", () => {
    const noClient = `${LD}const ld = init("x");\n`;
    expect(outcomes(`${noClient}ld.boolVariation("k", { key: u }, d); ld.boolVariation("k", { key: u }, false);`)).toEqual([
      "default-type-unproven",
      "unbound-client",
    ]);
  });

  it("keeps await and verbatim argument text, shorthand key", () => {
    const src = `${head}async function f(key: string) { return await ld.stringVariation('a', {key}, "x"); }`;
    expect(rewrite(src)).toBe(
      `${head}async function f(key: string) { return await client.getStringValue('a', "x", { targetingKey: key }); }`,
    );
  });

  it("accepts any literal default for object rules and numbers incl. negatives", () => {
    expect(outcomes(`${head}ld.jsonVariation("k", {key: u}, null); ld.jsonVariation("k", {key: u}, [1]); ld.numberVariation("k", {key: u}, -1);`)).toEqual([
      "rewrite",
      "rewrite",
      "rewrite",
    ]);
  });

  it("does not expand placeholders found inside substituted text", () => {
    const src = `${head}ld.stringVariation("{{key}}", {key: u}, "{{default}}");`;
    expect(rewrite(src)).toContain(`client.getStringValue("{{key}}", "{{default}}", { targetingKey: u })`);
  });

  it("only the outer call of a nested pair is rewritten in one pass", () => {
    const src = `${head}ld.boolVariation("a", { key: ld.boolVariation("b", { key: u }, false) ? "x" : "y" }, false);`;
    const once = rewrite(src);
    expect(once).toContain(`client.getBooleanValue("a", false, { targetingKey: ld.boolVariation(`);
    expect(rewrite(once)).toContain(`client.getBooleanValue("b", false`);
  });

  it("reports UTF-16 columns and tab = 1", () => {
    const f = analyze(`${head}const s = "😀é", on = \tld.boolVariation("k", { key: u }, false);`)[0]!;
    expect([f.site.line, f.site.col]).toEqual([4, 24]);
  });

  it("carries the decoded key value", () => {
    const [f] = analyze(`${head}ld.boolVariation('a\\u0041', { key: u }, false);`);
    expect(f?.site.keyValue).toBe("aA");
    expect(f?.site.keyText).toBe(`'a\\u0041'`);
  });
});
