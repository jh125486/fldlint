import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { validateRulesSemantics } from "../src/semantic.js";
import type { RulesFile } from "../src/types.js";
import { formatErrors, loadValidators } from "../src/validate.js";

const REAL_SPEC = resolve(dirname(fileURLToPath(import.meta.url)), "../../../../spec");
const validators = loadValidators(REAL_SPEC);

type Json = Record<string, unknown>;

const skips = ["detail", "bulk", "dynamic-key", "unbound-client", "type-mismatch", "context-unproven", "default-type-unproven", "receiver-unproven"];

/** A rules file with one rewriting rule and one report-only rule, after `edit`. */
function build(edit: (rewriting: Json, reportOnly: Json) => void = () => undefined): RulesFile {
  const rewriting: Json = {
    id: "bool-variation",
    message: "m {{key}}",
    valueType: "boolean",
    risk: "low",
    keyArg: 0,
    defaultArg: 2,
    contextArg: 1,
    languages: { js: { methods: ["boolVariation"], template: "{{client}}.f({{key}})" } },
  };
  const reportOnly: Json = {
    id: "variation",
    risk: "medium",
    keyArg: 0,
    skip: "type-mismatch",
    languages: { js: { methods: ["variation"], argCount: { min: 3, max: 4 } } },
  };
  edit(rewriting, reportOnly);
  return {
    version: 1,
    skipCategories: Object.fromEntries(skips.map((s) => [s, { severity: "warning", message: s }])),
    rules: [rewriting, reportOnly],
  } as unknown as RulesFile;
}

const schemaErrors = (rf: RulesFile): string => {
  const v = validators.rules!;
  return v(rf) ? "" : formatErrors(v);
};
const langs = (r: Json): Record<string, Json> => r.languages as Record<string, Json>;

describe("argCount schema", () => {
  it("accepts the base file, a variadic range and a Java binding", () => {
    expect(schemaErrors(build())).toBe("");
    expect(
      schemaErrors(
        build((_, ro) => {
          langs(ro).js!.argCount = { min: 1 };
          langs(ro).java = { methods: ["variation"], argCount: { min: 3, max: 3 } };
        }),
      ),
    ).toBe("");
  });

  const bad: { name: string; edit: (rw: Json, ro: Json) => void; want: string }[] = [
    { name: "missing on a report-only binding", edit: (_, ro) => delete langs(ro).js!.argCount, want: "argCount" },
    { name: "present on a rewriting rule", edit: (rw) => (langs(rw).js!.argCount = { min: 3, max: 3 }), want: "argCount" },
    { name: "min zero", edit: (_, ro) => (langs(ro).js!.argCount = { min: 0, max: 3 }), want: "must be >= 1" },
    { name: "max zero", edit: (_, ro) => (langs(ro).js!.argCount = { min: 1, max: 0 }), want: "must be >= 1" },
    { name: "min missing", edit: (_, ro) => (langs(ro).js!.argCount = { max: 3 }), want: "min" },
    { name: "non-integer min", edit: (_, ro) => (langs(ro).js!.argCount = { min: 1.5, max: 3 }), want: "integer" },
    { name: "unknown key", edit: (_, ro) => (langs(ro).js!.argCount = { min: 3, max: 3, exactly: 3 }), want: "additional properties" },
    { name: "not an object", edit: (_, ro) => (langs(ro).js!.argCount = 3), want: "object" },
  ];
  for (const { name, edit, want } of bad) {
    it(`rejects ${name}`, () => {
      expect(schemaErrors(build(edit))).toContain(want);
    });
  }
});

describe("argCount semantic checks", () => {
  const sem = (edit: (rw: Json, ro: Json) => void): string => validateRulesSemantics(build(edit)).join("\n");

  it("accepts a consistent range", () => {
    expect(sem(() => undefined)).toBe("");
    expect(sem((_, ro) => (langs(ro).js!.argCount = { min: 1 }))).toBe("");
  });

  it("rejects max below min", () => {
    expect(sem((_, ro) => (langs(ro).js!.argCount = { min: 4, max: 3 }))).toContain('rule "variation" js.argCount.max (3) must be >= min (4)');
  });

  it("rejects min that does not reach past keyArg", () => {
    expect(sem((_, ro) => (ro.keyArg = 3))).toContain('rule "variation" js.argCount.min (3) must be > keyArg (3)');
  });

  it("allows any min when the rule has no keyArg", () => {
    expect(sem((_, ro) => delete ro.keyArg)).toBe("");
  });

  it("rejects min below one even where the schema is not applied", () => {
    expect(sem((_, ro) => (langs(ro).js!.argCount = { min: 0, max: 3 }))).toContain("js.argCount.min must be >= 1");
  });
});

describe("the real rules.yaml", () => {
  const rf = (JSON.parse(readFileSync(join(REAL_SPEC, "generated/spec.json"), "utf8")) as { rules: RulesFile }).rules;
  it("gives every report-only binding an argCount and no rewriting binding one", () => {
    for (const r of rf.rules) {
      for (const [lang, b] of Object.entries(r.languages)) {
        expect(b.argCount !== undefined, `${r.id}/${lang}`).toBe(r.skip !== undefined);
      }
    }
  });
});
