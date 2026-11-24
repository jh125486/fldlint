import { describe, expect, it } from "vitest";
import { analyze, expand, rewrite, type CommentNotes } from "../src/index.js";

// Issue #31: comments inside a rewritten call move with the argument they
// touch; a comment with no home withholds the fix instead of being dropped.
// The Go engine has the same cases in go/internal/planner and the analyzer tests.

const PRE = [
  `import { init } from "@launchdarkly/node-server-sdk";`,
  `import { OpenFeature } from "@openfeature/server-sdk";`,
  `const ld = init("x");`,
  `const c = OpenFeature.getClient();`,
  ``,
].join("\n");

const BOM = String.fromCharCode(0xfeff);
const stmt = (call: string): string => `${PRE}const v = ${call};\n`;

describe("rewrite keeps comments inside the call", () => {
  const cases: { name: string; call: string; want: string }[] = [
    {
      name: "no comment: unchanged behaviour",
      call: `ld.boolVariation("k", { key: u }, false)`,
      want: `c.getBooleanValue("k", false, { targetingKey: u })`,
    },
    {
      name: "block comment after the key, before the comma, stays with the key",
      call: `ld.boolVariation("k" /* flag */, { key: u }, false)`,
      want: `c.getBooleanValue("k" /* flag */, false, { targetingKey: u })`,
    },
    {
      name: "block comment leading the context moves with it when arguments reorder",
      call: `ld.boolVariation("k", /* who */ { key: u }, false)`,
      want: `c.getBooleanValue("k", false, /* who */ { targetingKey: u })`,
    },
    {
      name: "block comment trailing the last argument moves with the default",
      call: `ld.boolVariation("k", { key: u }, false /* off */)`,
      want: `c.getBooleanValue("k", false /* off */, { targetingKey: u })`,
    },
    {
      name: "block comment before the first argument leads the key",
      call: `ld.boolVariation(/* flag */ "k", { key: u }, false)`,
      want: `c.getBooleanValue(/* flag */ "k", false, { targetingKey: u })`,
    },
    {
      name: "line comments after each argument move behind the comma that follows them",
      call: `ld.boolVariation(\n  "k", // the flag\n  { key: u }, // who\n  false, // default\n)`,
      want: `c.getBooleanValue("k", // the flag\n false, // default\n { targetingKey: u }, // who\n)`,
    },
    {
      name: "own-line comment above an argument leads it",
      call: `ld.boolVariation(\n  // the flag\n  "k",\n  { key: u },\n  false,\n)`,
      want: `c.getBooleanValue(// the flag\n"k", false, { targetingKey: u })`,
    },
    {
      name: "own-line comment after the last argument goes before the closing paren",
      call: `ld.boolVariation("k", { key: u }, false\n  // trailing note\n)`,
      want: `c.getBooleanValue("k", false, { targetingKey: u }, // trailing note\n)`,
    },
    {
      name: "comments between receiver, method and paren are hoisted before the new call",
      call: `ld /* a */ . /* b */ boolVariation /* p */ ("k", { key: u }, false)`,
      want: `/* a */ /* b */ /* p */ c.getBooleanValue("k", false, { targetingKey: u })`,
    },
    {
      name: "multi-line block comment trailing an argument is deferred behind the comma",
      call: `ld.boolVariation("k" /* a\n b */, { key: u }, false)`,
      want: `c.getBooleanValue("k", /* a\n b */ false, { targetingKey: u })`,
    },
    {
      name: "comment inside the key expression of the context is copied with it",
      call: `ld.boolVariation("k", { key: f(/* c */ u) }, false)`,
      want: `c.getBooleanValue("k", false, { targetingKey: f(/* c */ u) })`,
    },
    {
      name: "comments inside the default argument text are copied verbatim",
      call: `ld.jsonVariation("k", { key: u }, { a: /* c */ 1 })`,
      want: `c.getObjectValue("k", { a: /* c */ 1 }, { targetingKey: u })`,
    },
    {
      name: "CRLF: a line comment keeps its CRLF terminator",
      call: `ld.boolVariation("k", // note\r\n  { key: u }, false)`,
      want: `c.getBooleanValue("k", // note\r\n false, { targetingKey: u })`,
    },
  ];
  for (const { name, call, want } of cases) {
    it(name, () => {
      const src = stmt(call);
      const out = rewrite(src);
      expect(out).toBe(`${PRE}const v = ${want};\n`);
      // The result is clean: no LaunchDarkly call is left and a second run is a no-op.
      expect(rewrite(out)).toBe(out);
      expect(analyze(out)).toHaveLength(0);
    });
  }

  it("comments before and after the call, outside its range, are untouched", () => {
    const src = stmt(`/* before */ ld.boolVariation("k", { key: u }, false) /* after */`);
    expect(rewrite(src)).toBe(`${PRE}const v = /* before */ c.getBooleanValue("k", false, { targetingKey: u }) /* after */;\n`);
  });

  it("a leading BOM stays and does not shift where comments land", () => {
    const src = `${BOM}${stmt(`ld.boolVariation("k", /* who */ { key: u }, false)`)}`;
    expect(rewrite(src)).toBe(`${BOM}${PRE}const v = c.getBooleanValue("k", false, /* who */ { targetingKey: u });\n`);
  });

  it("an all-CRLF file keeps CRLF around a moved line comment", () => {
    const src = stmt(`ld.boolVariation(\n  "k", // flag\n  { key: u },\n  false,\n)`).replaceAll("\n", "\r\n");
    const crlf = (t: string): string => t.replaceAll("\n", "\r\n");
    expect(rewrite(src)).toBe(`${crlf(PRE)}const v = c.getBooleanValue("k", // flag\r\n false, { targetingKey: u });\r\n`);
  });
});

describe("a comment with no home withholds the fix", () => {
  const cases: { name: string; call: string }[] = [
    { name: "after the key inside the converted context literal", call: `ld.boolVariation("k", { key: u /* user id */ }, false)` },
    { name: "before the key inside the converted context literal", call: `ld.boolVariation("k", { key: /* c */ u }, false)` },
    { name: "next to kind inside the converted context literal", call: `ld.boolVariation("k", { key: u, /* c */ kind: "user" }, false)` },
    { name: "a line comment between receiver and method", call: `ld // note\n  .boolVariation("k", { key: u }, false)` },
    { name: "a ts-expect-error directive trailing an argument", call: `ld.boolVariation("k", { key: u }, false // @ts-expect-error\n)` },
    { name: "an eslint-disable directive before an argument", call: `ld.boolVariation(\n  /* eslint-disable-next-line x */\n  "k", { key: u }, false)` },
    { name: "a nolint directive between arguments", call: `ld.boolVariation("k", /* nolint */ { key: u }, false)` },
    { name: "a multi-line block comment between receiver and method", call: `ld /* a\n b */ .boolVariation("k", { key: u }, false)` },
  ];
  for (const { name, call } of cases) {
    it(name, () => {
      const src = stmt(call);
      const findings = analyze(src);
      expect(findings.map((f) => f.result.kind)).toEqual(["unfixable"]);
      expect(findings[0]?.result).toEqual({ kind: "unfixable", ruleId: "bool-variation" });
      expect(rewrite(src)).toBe(src);
    });
  }

  it("only the unfixable call is held back; its neighbour is still rewritten", () => {
    const src = `${PRE}const a = ld.boolVariation("a", { key: u /* x */ }, false);\nconst b = ld.boolVariation("b", { key: u }, false);\n`;
    expect(rewrite(src)).toBe(`${PRE}const a = ld.boolVariation("a", { key: u /* x */ }, false);\nconst b = c.getBooleanValue("b", false, { targetingKey: u });\n`);
  });

  it("a skip decided by an earlier step wins over a comment with no home", () => {
    const dynamic = stmt(`ld.boolVariation(k /* c */, { key: u /* x */ }, false)`);
    expect(analyze(dynamic).map((f) => f.result)).toEqual([{ kind: "skip", category: "dynamic-key", ruleId: "bool-variation" }]);
    const unbound = `${PRE.replace("const c = OpenFeature.getClient();", "")}const v = ld.boolVariation("k", { key: u /* x */ }, false);\n`;
    expect(analyze(unbound).map((f) => f.result.kind)).toEqual(["skip"]);
  });

  it("a call that is already skipped stays a skip, comments or not", () => {
    const src = stmt(`ld.boolVariation("k", ctx /* c */, false)`);
    expect(analyze(src).map((f) => f.result.kind)).toEqual(["skip"]);
  });
});

describe("expand", () => {
  const values = { client: "c", key: `"k"`, default: "false", context: "{ x }" };
  const tpl = "{{client}}.f({{key}}, {{default}}, {{context}})";
  const cm = (text: string, line = false): CommentNotes["head"][number] => ({ text, line, eol: line ? "\n" : "" });
  const empty = (): CommentNotes => ({ args: {}, head: [], tail: [], unhomed: false });

  it("equals plain substitution without notes", () => {
    expect(expand(tpl, values)).toBe(`c.f("k", false, { x })`);
    expect(expand(tpl, values, empty())).toBe(`c.f("k", false, { x })`);
  });
  it("leaves an unknown placeholder as written", () => {
    expect(expand("{{nope}} {{key}}", values)).toBe(`{{nope}} "k"`);
  });
  it("never re-expands substituted text", () => {
    expect(expand("{{key}}", { key: "{{client}}", client: "c" })).toBe("{{client}}");
  });
  it("decorates only the first occurrence of a placeholder", () => {
    const n = empty();
    n.args.key = { lead: [cm("/* a */")], trail: [] };
    expect(expand("{{key}} {{key}}", values, n)).toBe(`/* a */ "k" "k"`);
  });
  it("has no home for a comment bound to an argument the template does not use", () => {
    const n = empty();
    n.args.context = { lead: [cm("/* a */")], trail: [] };
    expect(expand("{{client}}.f({{key}})", values, n)).toBeUndefined();
  });
  it("has no home for a tail comment when the template has no placeholder", () => {
    expect(expand("f()", values, { ...empty(), tail: [cm("/* t */")] })).toBeUndefined();
  });
  it("has no home for a deferred comment when the template has no closing paren", () => {
    const n = empty();
    n.args.key = { lead: [], trail: [cm("// a", true)] };
    expect(expand("{{key}}", values, n)).toBeUndefined();
  });
  it("rejects a head comment that breaks the line", () => {
    expect(expand(tpl, values, { ...empty(), head: [cm("// a", true)] })).toBeUndefined();
  });
});
