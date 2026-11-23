import { describe, expect, it } from "vitest";
import { plan, type CallSite } from "../src/index.js";

const good: CallSite = {
  ruleId: "bool-variation",
  method: "boolVariation",
  range: [0, 10],
  line: 1,
  col: 1,
  keyIsStatic: true,
  keyText: `"k"`,
  keyValue: "k",
  defaultText: "false",
  defaultType: "boolean",
  contextShape: { kind: "simple", keyText: "u" },
  clientBinding: { kind: "bound", name: "c" },
};

// Each failing row also fails every later step, so only the order decides.
const bad = {
  dynamicKey: { keyIsStatic: false } as Partial<CallSite>,
  context: { contextShape: { kind: "other" } } as Partial<CallSite>,
  defaultType: { defaultType: "unknown" } as Partial<CallSite>,
  unbound: { clientBinding: { kind: "none" } } as Partial<CallSite>,
};

describe("plan step order", () => {
  const cases: { name: string; site: CallSite; want: string }[] = [
    {
      name: "rule-level skip beats dynamic-key, context, default and client",
      site: { ...good, ruleId: "bool-variation-detail", method: "boolVariationDetail", ...bad.dynamicKey, ...bad.context, ...bad.defaultType, ...bad.unbound },
      want: "detail",
    },
    { name: "dynamic-key beats context, default and client", site: { ...good, ...bad.dynamicKey, ...bad.context, ...bad.defaultType, ...bad.unbound }, want: "dynamic-key" },
    { name: "missing keyText counts as dynamic", site: { ...good, keyText: undefined as never, ...bad.context, ...bad.unbound }, want: "dynamic-key" },
    { name: "context-unproven beats default and client", site: { ...good, ...bad.context, ...bad.defaultType, ...bad.unbound }, want: "context-unproven" },
    { name: "default-type-unproven beats an unbound client", site: { ...good, ...bad.defaultType, ...bad.unbound }, want: "default-type-unproven" },
    { name: "default-type-unproven beats an ambiguous client", site: { ...good, ...bad.defaultType, clientBinding: { kind: "ambiguous" } }, want: "default-type-unproven" },
    { name: "wrong default type beats an unbound client", site: { ...good, defaultType: "string", ...bad.unbound }, want: "default-type-unproven" },
    { name: "unbound-client when only the client fails", site: { ...good, ...bad.unbound }, want: "unbound-client" },
    { name: "ambiguous client is unbound-client", site: { ...good, clientBinding: { kind: "ambiguous" } }, want: "unbound-client" },
  ];
  for (const { name, site, want } of cases) {
    it(name, () => {
      const r = plan(site);
      expect(r.kind === "skip" ? r.category : r.kind).toBe(want);
    });
  }

  it("rewrites when every fact holds", () => {
    expect(plan(good)).toEqual({
      kind: "rewrite",
      edit: { range: [0, 10], text: `c.getBooleanValue("k", false, { targetingKey: u })` },
    });
  });
  it("throws on an unknown rule id", () => {
    expect(() => plan({ ...good, ruleId: "nope" })).toThrow(/unknown rule/);
  });
});
