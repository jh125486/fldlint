import { describe, expect, it, vi } from "vitest";

vi.mock("@fldlint/spec", async (importOriginal) => {
  const orig = await importOriginal<typeof import("@fldlint/spec")>();
  return {
    ...orig,
    findRuleByMethod: (lang: "js" | "go" | "java", method: string) =>
      lang === "js" && method === "boolVariation"
        ? { rule: { id: "bool-variation", risk: "low", valueType: "boolean", keyArg: 0, defaultArg: 2, contextArg: 1, languages: { js: { methods: ["boolVariation"] } } }, language: { methods: ["boolVariation"] } }
        : orig.findRuleByMethod(lang, method),
  };
});

const { analyze } = await import("../src/index.js");

describe("a rewriting rule that has JS methods but no JS template", () => {
  it("is ignored by detection and never throws", () => {
    const src = `import { init } from "@launchdarkly/node-server-sdk";\nconst ld = init("x");\nld.boolVariation("k", { key: u }, false);`;
    expect(analyze(src)).toEqual([]);
  });
});
