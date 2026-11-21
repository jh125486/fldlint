import { describe, expect, it } from "vitest";
import { findRuleByMethod, plan } from "../src/index.js";

describe("findRuleByMethod", () => {
  it("finds a rule by language method", () => {
    expect(findRuleByMethod("js", "boolVariation")?.rule.id).toBe("bool-variation");
  });
  it("returns undefined for unknown methods and wrong language", () => {
    expect(findRuleByMethod("js", "nope")).toBeUndefined();
    expect(findRuleByMethod("go", "boolVariation")).toBeUndefined();
  });
});

describe("plan (stub)", () => {
  it("always skips with unbound-client", () => {
    expect(plan({ language: "js", method: "boolVariation" })).toEqual({
      kind: "skip",
      category: "unbound-client",
      ruleId: "bool-variation",
    });
  });
});
