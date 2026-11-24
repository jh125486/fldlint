import { mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";
import { discoverCases, loadYaml } from "../src/load.js";
import { STUB_SPEC_ROOT } from "../src/specRoot.js";
import { validateSpec } from "../src/validate.js";
import { classifyPlain, decodeUtf8, MAX_DEPTH, parseSubset, YamlSubsetError } from "../src/yamlSubset.js";
import type { PlainClass, YamlSubsetErrorCode } from "../src/yamlSubset.js";

const here = dirname(fileURLToPath(import.meta.url));
const REPO_SPEC = resolve(here, "../../../../spec");
const PROBES = join(REPO_SPEC, "probes/yaml");
// Minimums stated in spec/probes/README.md ("at least MIN_REJECT reject-*.yaml and at least MIN_ACCEPT accept-*.yaml").
const MIN_REJECT = 102;
const MIN_ACCEPT = 62;

function rejection(text: string): YamlSubsetError {
  try {
    parseSubset(text);
  } catch (e) {
    if (e instanceof YamlSubsetError) return e;
    throw e;
  }
  throw new Error(`expected a YamlSubsetError for ${JSON.stringify(text)}`);
}

const str = (value: string): PlainClass => ({ kind: "string", value });
const int = (value: number): PlainClass => ({ kind: "integer", value });
const amb = (rule: "R4a" | "R4b" | "R4c" | "R4d" | "R4e" | "R4f"): PlainClass => ({ kind: "ambiguous", rule });
const OOR: PlainClass = { kind: "out-of-range" };
const NULL: PlainClass = { kind: "null" };

describe("classifyPlain", () => {
  const table: [label: string, input: string, expected: PlainClass][] = [
    // R1
    ["R1", "true", { kind: "boolean", value: true }],
    ["R1", "false", { kind: "boolean", value: false }],
    ["R1 near miss: title case", "True", amb("R4a")],
    ["R1 near miss: upper case", "FALSE", amb("R4a")],
    ["R1 near miss: prefix", "truex", str("truex")],
    // R2
    ["R2", "0", int(0)],
    ["R2", "7", int(7)],
    ["R2", "-7", int(-7)],
    ["R2", "9007199254740991", int(9007199254740991)],
    ["R2", "-9007199254740991", int(-9007199254740991)],
    ["R2 above safe", "9007199254740992", OOR],
    ["R2 below safe", "-9007199254740992", OOR],
    ["R2 very long", "1" + "0".repeat(400), OOR],
    ["R2 near miss: leading zero", "017", amb("R4b")],
    ["R2 near miss: plus sign", "+1", amb("R4b")],
    ["R2 near miss: negative zero", "-0", amb("R4b")],
    ["R2 near miss: 00", "00", amb("R4b")],
    // R3
    ["R3", "", NULL],
    ["R3", "~", NULL],
    ["R3", "null", NULL],
    ["R3", "Null", NULL],
    ["R3", "NULL", NULL],
    ["R3 near miss: odd casing", "nULL", amb("R4a")],
    ["R3 near miss: double tilde", "~~", str("~~")],
    // R4a
    ["R4a", "yes", amb("R4a")],
    ["R4a", "Yes", amb("R4a")],
    ["R4a", "NO", amb("R4a")],
    ["R4a", "y", amb("R4a")],
    ["R4a", "N", amb("R4a")],
    ["R4a", "On", amb("R4a")],
    ["R4a", "OFF", amb("R4a")],
    ["R4a", "=", amb("R4a")],
    ["R4a", "<<", amb("R4a")],
    ["R4a near miss", "yesterday", str("yesterday")],
    ["R4a near miss", "nope", str("nope")],
    ["R4a near miss", "online", str("online")],
    ["R4a near miss", "==", str("==")],
    // R4b
    ["R4b", "1.0", amb("R4b")],
    ["R4b", ".5", amb("R4b")],
    ["R4b", "1.", amb("R4b")],
    ["R4b", "1e3", amb("R4b")],
    ["R4b", "1E-3", amb("R4b")],
    ["R4b", "-1.5e+3", amb("R4b")],
    ["R4b", "1_0", amb("R4b")],
    ["R4b", "1,000", amb("R4b")],
    ["R4b", "+.5", amb("R4b")],
    ["R4b near miss: two dots", "1.2.3", str("1.2.3")],
    ["R4b near miss: dotted quad", "10.0.0.1", str("10.0.0.1")],
    ["R4b near miss: digit prefix", "1st", str("1st")],
    ["R4b near miss: lone dot", ".", str(".")],
    ["R4b near miss: lone minus", "-", str("-")],
    ["R4b near miss: dot e", ".e5", str(".e5")],
    // R4c
    ["R4c", ".inf", amb("R4c")],
    ["R4c", "-.Inf", amb("R4c")],
    ["R4c", "+.INF", amb("R4c")],
    ["R4c", ".nan", amb("R4c")],
    ["R4c", ".NaN", amb("R4c")],
    ["R4c near miss", "inf", str("inf")],
    ["R4c near miss", ".infx", str(".infx")],
    // R4d
    ["R4d", "0x1F", amb("R4d")],
    ["R4d", "0X1f", amb("R4d")],
    ["R4d", "-0x1", amb("R4d")],
    ["R4d", "0b101", amb("R4d")],
    ["R4d", "0o17", amb("R4d")],
    ["R4d", "+0xa_b", amb("R4d")],
    ["R4d near miss: prefix alone", "0x", str("0x")],
    ["R4d near miss: non-hex digit", "0xg", str("0xg")],
    ["R4d near miss: other letter", "0z1", str("0z1")],
    // R4e
    ["R4e", "1:30", amb("R4e")],
    ["R4e", "190:20:30", amb("R4e")],
    ["R4e", "-1:30.5", amb("R4e")],
    ["R4e", "1_0:59", amb("R4e")],
    ["R4e near miss: minutes out of range", "1:60", str("1:60")],
    ["R4e near miss: letters", "a:30", str("a:30")],
    // R4f
    ["R4f", "2001-12-14", amb("R4f")],
    ["R4f", "2001-1-1", amb("R4f")],
    ["R4f", "2001-12-14T21:59:43Z", amb("R4f")],
    ["R4f", "2001-12-14t21:59", amb("R4f")],
    ["R4f", "2001-12-14 21:59:43", amb("R4f")],
    ["R4f", "2001-12-14 is a date", amb("R4f")],
    ["R4f near miss: short year", "200-12-14", str("200-12-14")],
    ["R4f near miss: trailing text", "2001-12-14x", str("2001-12-14x")],
    ["R4f near miss: three-digit month", "2001-123-14", str("2001-123-14")],
    // R5
    ["R5", "hello", str("hello")],
    ["R5", "hello world", str("hello world")],
    ["R5", "3 flags", str("3 flags")],
    ["R5", "1.2.3-beta", str("1.2.3-beta")],
    ["R5", "héllo", str("héllo")],
    // Case-insensitive rules fold ASCII letters only: U+017F (long s) and U+212A (Kelvin sign) fold to s and k in Unicode, not here.
    ["R4a ASCII fold only", "falſe", str("falſe")],
    ["R4a ASCII fold only", "yeſ", str("yeſ")],
    ["R4a ASCII fold only", "ſ", str("ſ")],
    ["R4a ASCII fold only", "ＴＲＵＥ", str("ＴＲＵＥ")],
    ["R4d ASCII fold only", "0xK", str("0xK")],
    ["R4c ASCII fold only", ".İnf", str(".İnf")],
    // R4f is a prefix rule, so text after the date may be non-ASCII.
    ["R4f prefix, non-ASCII tail", "2001-12-14 café", amb("R4f")],
    // Texts the YAML 1.1 parsers type as numbers although no rule matches them as one: strings here.
    ["R5 look-alike", "0e0_", str("0e0_")],
    ["R5 look-alike", "1e_0", str("1e_0")],
    ["R5 look-alike", "0o-1", str("0o-1")],
    ["R5 look-alike", "0b+1", str("0b+1")],
    ["R5 look-alike", "0_x1", str("0_x1")],
  ];

  it.each(table)("%s: %j", (_label, input, expected) => {
    expect(classifyPlain(input)).toEqual(expected);
  });

  it("anchors every rule at both ends (a trailing newline is not part of the scalar)", () => {
    expect(classifyPlain("1\n")).toEqual(str("1\n"));
    expect(classifyPlain("x1")).toEqual(str("x1"));
  });
});

describe("parseSubset: values", () => {
  it("types plain scalars by the contract and keeps quoted and block scalars as strings", () => {
    const text = [
      "a: true",
      "b: 12",
      "c: ~",
      "d: plain text",
      'e: "017"',
      "f: 'yes'",
      "g: |",
      "  123",
      "h: >",
      "  null",
      "i:",
      "",
    ].join("\n");
    expect(parseSubset(text)).toEqual({ a: true, b: 12, c: null, d: "plain text", e: "017", f: "yes", g: "123\n", h: "null\n", i: null });
  });

  it("folds multi-line plain scalars before classifying them", () => {
    expect(parseSubset("a: one\n  two\n  three\n")).toEqual({ a: "one two three" });
    expect(rejection("a: 2001-12-14\n  is a date\n").code).toBe("ambiguous-scalar");
  });

  it("handles sequences, flow collections, nesting and keys without values", () => {
    expect(parseSubset("- 1\n- [a, {b: 2}]\n- {}\n- ~\n")).toEqual([1, ["a", { b: 2 }], {}, null]);
    expect(parseSubset("{a, b: }")).toEqual({ a: null, b: null });
    expect(parseSubset("[a: 1, b]")).toEqual([{ a: 1 }, "b"]);
  });

  it("accepts an explicit start marker, comments, BOM and CRLF", () => {
    expect(parseSubset("---\na: 1\n")).toEqual({ a: 1 });
    expect(parseSubset("# c\n\n  # d\na: 1 # e\n")).toEqual({ a: 1 });
    expect(parseSubset("\uFEFFa: 1\n")).toEqual({ a: 1 });
    expect(parseSubset("a: 1\r\nb:\r\n  - x\r\n  - 2\r\n")).toEqual({ a: 1, b: ["x", 2] });
    expect(parseSubset("\uFEFF# c\r\na: 1\r\n")).toEqual({ a: 1 });
  });

  it("returns null for a bare start marker, a document whose content is null", () => {
    expect(parseSubset("---\n")).toBeNull();
    expect(parseSubset("---")).toBeNull();
    expect(parseSubset("--- # c\n")).toBeNull();
    expect(parseSubset("---\n...\n")).toBeNull();
    expect(parseSubset("\uFEFF---\n")).toBeNull();
    expect(parseSubset("# c\n---\n# d\n")).toBeNull();
  });

  it("accepts one document end marker, then only blank lines and comments", () => {
    expect(parseSubset("a: 1\n...\n")).toEqual({ a: 1 });
    expect(parseSubset("a: 1\n... # c\n\n# d\n")).toEqual({ a: 1 });
    expect(parseSubset("a: ...\nb: x...\n")).toEqual({ a: "...", b: "x..." });
  });

  it("reads quoted scalars inside flow collections", () => {
    expect(parseSubset('a: ["x", {"k": \'v\'}, \'it\'\'s\', "a\\"b"]\n')).toEqual({ a: ["x", { k: "v" }, "it's", 'a"b'] });
    expect(parseSubset("a: ['x\n  y',\n  z]\n")).toEqual({ a: ["x y", "z"] });
    expect(parseSubset('a: {b, "c": 1}\n')).toEqual({ a: { b: null, c: 1 } });
  });

  it("lets a quoted key be followed directly by its value in a flow collection, and a plain scalar contain a colon", () => {
    expect(parseSubset('a: {"k":1}\nb: ["x:", \'y:\', {"z":[]}]\n')).toEqual({ a: { k: 1 }, b: ["x:", "y:", { z: [] }] });
    expect(parseSubset("a: [x:y, {p: q:r}]\n")).toEqual({ a: ["x:y", { p: "q:r" }] });
  });

  it("treats a missing final line break as present", () => {
    expect(parseSubset("a: >\n  x\n  y")).toEqual({ a: "x y\n" });
    expect(parseSubset("a: |\n  x")).toEqual({ a: "x\n" });
    expect(parseSubset("a: 1")).toEqual({ a: 1 });
  });

  it("accepts nesting up to the limit", () => {
    const text = Array.from({ length: MAX_DEPTH }, (_, i) => `${" ".repeat(i)}k${String(i)}:`).join("\n") + " leaf\n";
    let v: unknown = parseSubset(text);
    for (let i = 0; i < MAX_DEPTH; i++) v = (v as Record<string, unknown>)[`k${String(i)}`];
    expect(v).toBe("leaf");
    const flow = `${"[".repeat(MAX_DEPTH)}1${"]".repeat(MAX_DEPTH)}`;
    let f: unknown = parseSubset(flow);
    for (let i = 0; i < MAX_DEPTH - 1; i++) f = (f as unknown[])[0];
    expect(f).toEqual([1]);
  });

  it("stores every key as an ordinary own property (no prototype surprises)", () => {
    const v = parseSubset("__proto__: {polluted: yes_}\nconstructor: 1\ntoString: 2\nhasOwnProperty: 3\n") as Record<string, unknown>;
    expect(Object.getPrototypeOf(v)).toBe(Object.prototype);
    expect(Object.keys(v)).toEqual(["__proto__", "constructor", "toString", "hasOwnProperty"]);
    const own = (k: string): unknown => Object.getOwnPropertyDescriptor(v, k)?.value;
    expect(own("__proto__")).toEqual({ polluted: "yes_" });
    expect(own("constructor")).toBe(1);
    expect(own("toString")).toBe(2);
    expect(Object.hasOwn({}, "polluted")).toBe(false);
    expect(JSON.parse(JSON.stringify(v)) as unknown).toEqual({ ["__proto__"]: { polluted: "yes_" }, constructor: 1, toString: 2, hasOwnProperty: 3 });
    expect(rejection("__proto__: 1\n__proto__: 2\n").code).toBe("duplicate-key");
  });

  it("resolves every scalar to a string in failsafe mode (what plainText relies on)", () => {
    expect(parseSubset('a: "1"\nb: |\n  2\n')).toEqual({ a: "1", b: "2\n" });
  });

  it("is near-linear on large inputs", () => {
    const many = Array.from({ length: 20000 }, (_, i) => `k${String(i)}:\n  v${String(i)}\n`).join("");
    const long = `a: ${"x ".repeat(500000)}\n`;
    const start = performance.now();
    expect(Object.keys(parseSubset(many) as object)).toHaveLength(20000);
    expect(parseSubset(long)).toEqual({ a: "x ".repeat(500000).trim() });
    expect(performance.now() - start).toBeLessThan(10000);
  });
});

describe("parseSubset: rejections", () => {
  type Row = [name: string, text: string, code: YamlSubsetErrorCode, line: number, col: number];
  const rows: Row[] = [
    ["empty input", "", "empty-document", 1, 1],
    ["blank lines", "\n  \n\n", "empty-document", 1, 1],
    ["comment only", "# only a comment\n", "empty-document", 1, 1],
    ["comments and blanks", "\n# a\n  # b\n\n", "empty-document", 1, 1],
    ["BOM only", "\uFEFF", "empty-document", 1, 1],
    ["BOM and comment", "\uFEFF# c\r\n", "empty-document", 1, 1],
    ["document end marker only", "...\n", "empty-document", 1, 1],
    ["document end marker only, no line break", "...", "empty-document", 1, 1],
    ["directive", "%YAML 1.1\n---\na: 1\n", "directive", 1, 1],
    ["directive after a start marker is a syntax error", "---\n%YAML 1.2\na: 1\n", "syntax", 2, 1],
    ["indented directive is a syntax error", " %YAML 1.2\na: 1\n", "syntax", 1, 2],
    ["directive after comments and blanks", "# c\n\n  # d\n%TAG ! x\na: 1\n", "directive", 4, 1],
    ["directive after BOM", "\uFEFF%YAML 1.2\na: 1\n", "directive", 1, 1],
    ["directive with CRLF", "# c\r\n%YAML 1.2\r\na: 1\r\n", "directive", 2, 1],
    ["lone CR", "a: 1\rb: 2\n", "character", 1, 5],
    ["lone CR at the end", "a: 1\r", "character", 1, 5],
    ["NEL", "# c\u0085%YAML 1.2\na: 1\n", "character", 1, 4],
    ["line separator", "a: 1\u2028b: 2\n", "character", 1, 5],
    ["paragraph separator", "a: \"x\u2029y\"\n", "character", 1, 6],
    ["form feed", "a: 1\n\f\nb: 2\n", "character", 2, 1],
    ["NUL", "a: \"x\0y\"\n", "character", 1, 6],
    ["DEL", "a: \"x\x7Fy\"\n", "character", 1, 6],
    ["C1 control", "a: \"x\u0080y\"\n", "character", 1, 6],
    ["BOM inside the text", "a: \"x\uFEFFy\"\n", "character", 1, 6],
    ["BOM twice", "\uFEFF\uFEFFa: 1\n", "character", 1, 1],
    ["noncharacter", "a: \"x\uFFFEy\"\n", "character", 1, 6],
    ["lone surrogate", "a: \"x\uD800y\"\n", "character", 1, 6],
    ["astral character is fine but its position counts code units", "k: \u{1F600}\nb: \"\u0001\"\n", "character", 2, 5],
    ["tab-indented comment", "\t# c\na: 1\n", "tab-indent", 1, 1],
    ["tab-indented blank line", "a: 1\n  \t\nb: 2\n", "tab-indent", 2, 3],
    ["tab-indented block", "a:\n\t- b\n", "tab-indent", 2, 1],
    ["tab after a dash", "- \tx\n", "tab-indent", 1, 3],
    ["tab after a question mark", "?\ta\n", "tab-indent", 1, 2],
    ["tab after nested dashes", "- - \tx\n", "tab-indent", 1, 5],
    ["tab inside a quoted continuation line", "a: \"x\n\ty\"\n", "tab-indent", 2, 1],
    ["nesting too deep", `a: ${"[".repeat(MAX_DEPTH)}${"]".repeat(MAX_DEPTH)}\n`, "too-deep", 1, 3 + MAX_DEPTH],
    ["nesting too deep in blocks", `${Array.from({ length: MAX_DEPTH + 1 }, (_, i) => `${" ".repeat(i)}a:`).join("\n")}\n${" ".repeat(MAX_DEPTH + 1)}b: 1\n`, "too-deep", MAX_DEPTH + 1, MAX_DEPTH + 1],
    ["block scalar indentation indicator", "a: |2\n   x\n", "block-indicator", 1, 4],
    ["block scalar indentation indicator, folded with chomping", "a: >-1\n  x\n", "block-indicator", 1, 4],
    ["block scalar indentation indicator after chomping", "a: |1+\n  x\n", "block-indicator", 1, 4],
    ["flow scalar: trailing colon", "a: [b:]\n", "flow-scalar", 1, 5],
    ["flow scalar: trailing colon in a flow mapping", "{b:}", "flow-scalar", 1, 2],
    ["flow scalar: key glued to a flow value", "a: {b:[1]}\n", "flow-scalar", 1, 5],
    ["flow scalar: trailing colon after white space", "[b :]", "flow-scalar", 1, 2],
    ["flow scalar: trailing colon in a nested flow mapping", "- {a: [x, z:]}\n", "flow-scalar", 1, 11],
    ["syntax error", "a: [1\n", "syntax", 2, 1],
    ["syntax error mid-line", "a: 1\n b: 2\n", "syntax", 1, 4],
    ["alias", "a: *x\n", "alias", 1, 4],
    ["alias after a quoted key on a later line", "a: { 'q'\n  *a}\n", "syntax", 2, 3],
    ["two documents", "a: 1\n---\nb: 2\n", "multiple-documents", 2, 1],
    ["trailing document marker", "a: 1\n---\n", "multiple-documents", 2, 1],
    ["content after the document end marker", "a: 1\n...\nb: 2\n", "multiple-documents", 3, 1],
    ["tag !!str", "a: !!str 5\n", "tag", 1, 4],
    ["tag custom", "a: !custom 5\n", "tag", 1, 4],
    ["tag non-specific", "a: ! b\n", "tag", 1, 4],
    ["tag on a mapping", "a: !!map\n  b: 1\n", "tag", 1, 4],
    ["tag on a sequence", "a: !!seq\n  - 1\n", "tag", 1, 4],
    ["tag on a key", "!!str a: 1\n", "tag", 1, 1],
    ["tag on the root mapping", "!!map\na: 1\n", "tag", 1, 1],
    ["tag after a quoted key on a later line", "a: { 'q'\n  !!str}\n", "tag", 2, 3],
    ["tag after a quoted key on the same line", "a: { 'q'\t!x}\n", "tag", 1, 10],
    ["anchor", "a: &x 1\n", "anchor", 1, 4],
    ["anchor on a key", "&k a: 1\n", "anchor", 1, 1],
    ["anchor reported before its alias", "a: &x 1\nb: *x\n", "anchor", 1, 4],
    ["anchor after a quoted key on a later line", "a: { 'q'\n  &a}\n", "anchor", 2, 3],
    ["anchor glued to a quoted scalar", "a: { 'q'&a}\n", "anchor", 1, 9],
    ["complex key", "{[a]: 1}", "complex-key", 1, 2],
    ["complex mapping key", "{{a: 1}: 1}", "complex-key", 1, 2],
    ["integer key", "1: x\n", "key-not-string", 1, 1],
    ["boolean key", "a: 1\ntrue: x\n", "key-not-string", 2, 1],
    ["null key", "~: x\n", "key-not-string", 1, 1],
    ["empty key", ": x\n", "key-not-string", 1, 1],
    ["leading-zero key", "017: x\n", "ambiguous-scalar", 1, 1],
    ["ambiguous key", "yes: x\n", "ambiguous-scalar", 1, 1],
    ["duplicate key", "a: 1\nb: 2\na: 3\n", "duplicate-key", 3, 1],
    ["duplicate key, one quoted", "a: 1\n\"a\": 3\n", "duplicate-key", 2, 1],
    ["duplicate key, nested", "x:\n  a: 1\n  a: 2\n", "duplicate-key", 3, 3],
    ["duplicate key in a flow mapping", "{a: 1, a: 2}", "duplicate-key", 1, 8],
    ["ambiguous scalar", "a: 017\n", "ambiguous-scalar", 1, 4],
    ["ambiguous scalar in a sequence", "- ok\n- yes\n", "ambiguous-scalar", 2, 3],
    ["ambiguous date", "a: b\nc:\n  d: 2001-12-14\n", "ambiguous-scalar", 3, 6],
    ["out-of-range integer", "a: 9007199254740992\n", "integer-out-of-range", 1, 4],
    ["out-of-range negative integer", "- -9007199254740992\n", "integer-out-of-range", 1, 3],
    ["out-of-range as a key", "9007199254740992: x\n", "integer-out-of-range", 1, 1],
  ];

  it.each(rows)("%s", (_name, text, code, line, col) => {
    const err = rejection(text);
    expect({ code: err.code, line: err.line, column: err.column }).toEqual({ code, line, column: col });
    expect(err.message).toBe(`line ${String(line)}, col ${String(col)}: ${err.reason}`);
    expect(err.name).toBe("YamlSubsetError");
    expect(err).toBeInstanceOf(Error);
    expect(err.reason.length).toBeGreaterThan(0);
  });

  it("reports multi-line scalars at the line where the scalar starts", () => {
    const err = rejection("a: 1\nb: 2001-12-14\n  more\n");
    expect([err.line, err.column]).toEqual([2, 4]);
  });

  it("counts columns in UTF-16 code units and ignores a leading BOM", () => {
    expect([rejection("k: \u{1F600}x\n\u{1F600}: 017\n").line]).toEqual([2]);
    const astral = rejection('"\u{1F600}": 017\n');
    expect(astral.column).toBe(7); // quote, two code units, quote, colon, space -> value at index 6
    expect(rejection("\uFEFFa: 017\n").column).toBe(4);
  });

  it("names the rule and quotes the scalar in ambiguity messages", () => {
    expect(rejection("a: 017\n").reason).toBe('ambiguous plain scalar "017" (R4b); quote it');
    expect(rejection("a: 9007199254740992\n").reason).toBe("integer 9007199254740992 is outside the safe range; quote it");
  });

  it("strips the library's position suffix from syntax messages", () => {
    const err = rejection("a: [1\n");
    expect(err.reason).not.toMatch(/ at line /);
    expect(err.reason).not.toContain("\n");
  });
});

/**
 * The error a reject probe must fail with, from the name prefix; the first
 * matching prefix wins (spec/probes/README.md lists the same table), so a probe
 * fails for the reason its name says.
 */
const PROBE_CODES: [prefix: string, code: YamlSubsetErrorCode][] = [
  ["reject-empty-", "empty-document"],
  ["reject-directive-after-start-marker", "syntax"],
  ["reject-directive-indented", "syntax"],
  ["reject-directive-", "directive"],
  ["reject-invalid-utf8", "encoding"],
  ["reject-surrogate-utf8", "encoding"],
  ["reject-bom-", "character"],
  ["reject-lone-cr", "character"],
  ["reject-nel-", "character"],
  ["reject-line-separator-", "character"],
  ["reject-paragraph-separator-", "character"],
  ["reject-form-feed", "character"],
  ["reject-nul", "character"],
  ["reject-delete-char", "character"],
  ["reject-tab-", "tab-indent"],
  ["reject-tag-", "tag"],
  ["reject-anchor", "anchor"],
  ["reject-alias", "anchor"], // an alias needs an anchor, which is rejected first
  ["reject-key-complex-", "complex-key"],
  ["reject-key-", "key-not-string"],
  ["reject-duplicate-keys", "duplicate-key"],
  ["reject-two-documents", "multiple-documents"],
  ["reject-trailing-document-marker", "multiple-documents"],
  ["reject-too-deep", "too-deep"],
  ["reject-block-indicator", "block-indicator"],
  ["reject-flow-scalar-", "flow-scalar"],
  ["reject-out-of-range-", "integer-out-of-range"],
  ["reject-ambiguous-", "ambiguous-scalar"],
];

describe("shared probes (spec/probes/yaml)", () => {
  const files = readdirSync(PROBES).sort();
  const rejects = files.filter((f) => f.startsWith("reject-") && f.endsWith(".yaml"));
  const accepts = files.filter((f) => f.startsWith("accept-") && f.endsWith(".yaml"));
  const sidecars = files.filter((f) => f.startsWith("accept-") && f.endsWith(".json"));

  it("has no unexpected files and meets the README minimums", () => {
    const known = new Set([...rejects, ...accepts, ...sidecars]);
    expect(files.filter((f) => !known.has(f))).toEqual([]);
    expect(rejects.length).toBeGreaterThanOrEqual(MIN_REJECT);
    expect(accepts.length).toBeGreaterThanOrEqual(MIN_ACCEPT);
  });

  it("pairs every accept probe with exactly one json sidecar", () => {
    expect(sidecars.map((f) => f.replace(/\.json$/, "")).sort()).toEqual(accepts.map((f) => f.replace(/\.yaml$/, "")).sort());
  });

  // Through loadYaml, the loader entry point, so that the bytes are decoded as the loader decodes them.
  it.each(rejects)("%s is rejected for the reason its name says", (f) => {
    const entry = PROBE_CODES.find(([prefix]) => f.startsWith(prefix));
    expect(entry, `no entry in PROBE_CODES matches ${f}`).toBeDefined();
    const err = (() => {
      try {
        loadYaml(join(PROBES, f));
      } catch (e) {
        return e;
      }
      return undefined;
    })();
    expect(err).toBeInstanceOf(YamlSubsetError);
    expect((err as YamlSubsetError).code).toBe(entry?.[1]);
  });

  it.each(accepts)("%s loads to its sidecar", (f) => {
    const expected = JSON.parse(readFileSync(join(PROBES, f.replace(/\.yaml$/, ".json")), "utf8")) as unknown;
    expect(loadYaml(join(PROBES, f))).toStrictEqual(expected);
  });

  it("pins the nesting limit exactly", () => {
    const levels = (f: string): number => {
      let depth = 0;
      let max = 0;
      for (const c of readFileSync(join(PROBES, f), "utf8")) {
        if (c === "[") max = Math.max(max, ++depth + 1);
        else if (c === "]") depth--;
      }
      return max; // the root mapping is level 1
    };
    expect(levels("reject-too-deep.yaml")).toBe(MAX_DEPTH + 1);
    expect(levels("accept-depth-100.yaml")).toBe(MAX_DEPTH);
  });
});

describe("loadYaml integration", () => {
  const tmp = (): string => mkdtempSync(join(tmpdir(), "fldlint-subset-"));

  it("rejects a subset violation with a positioned message", () => {
    const dir = tmp();
    const file = join(dir, "x.yaml");
    writeFileSync(file, "a: 1\ncol: 017\n");
    expect(() => loadYaml(file)).toThrow(YamlSubsetError);
    expect(() => loadYaml(file)).toThrow(/^line 2, col 6: ambiguous plain scalar "017"/);
  });

  it("rejects input that is not valid UTF-8 instead of repairing it", () => {
    const dir = tmp();
    const file = join(dir, "x.yaml");
    for (const bytes of [[0x61, 0x3a, 0x20, 0xff, 0x0a], [0x61, 0x3a, 0x20, 0xed, 0xa0, 0x80, 0x0a], [0xc0, 0xaf], [0xe2, 0x82]]) {
      writeFileSync(file, Uint8Array.from(bytes));
      expect(() => loadYaml(file)).toThrow(/^line 1, col 1: input is not valid UTF-8$/);
      expect(() => decodeUtf8(Uint8Array.from(bytes))).toThrow(YamlSubsetError);
    }
    writeFileSync(file, Uint8Array.from([0xef, 0xbb, 0xbf, 0x61, 0x3a, 0x20, 0x31, 0x0a]));
    expect(loadYaml(file)).toEqual({ a: 1 });
    expect(decodeUtf8(Uint8Array.from([0xef, 0xbb, 0xbf]))).toBe("\uFEFF");
  });

  it("reports an empty file", () => {
    const dir = tmp();
    const file = join(dir, "x.yaml");
    for (const text of ["", "\n", "# c\n"]) {
      writeFileSync(file, text);
      expect(() => loadYaml(file)).toThrow(/^line 1, col 1: empty document: no content$/);
    }
    writeFileSync(file, "---\n");
    expect(loadYaml(file)).toBeNull();
  });

  it("surfaces an empty case.yaml as CaseFile.parseError", () => {
    const root = tmp();
    mkdirSync(join(root, "fixtures/r/c"), { recursive: true });
    writeFileSync(join(root, "fixtures/r/c/case.yaml"), "# nothing here\n");
    const cases = discoverCases(root);
    expect(cases[0]?.data).toBeUndefined();
    expect(cases[0]?.parseError).toBe("line 1, col 1: empty document: no content");
  });

  it("surfaces an empty rules.yaml through validateSpec", () => {
    const root = tmp();
    mkdirSync(join(root, "schema"), { recursive: true });
    for (const s of ["rules.schema.json", "fixture.schema.json"]) {
      writeFileSync(join(root, "schema", s), readFileSync(join(STUB_SPEC_ROOT, "schema", s)));
    }
    mkdirSync(join(root, "fixtures"), { recursive: true });
    writeFileSync(join(root, "rules.yaml"), "");
    expect(validateSpec(root).errors).toContain("rules.yaml: line 1, col 1: empty document: no content");
  });

  it("surfaces a case.yaml violation as CaseFile.parseError", () => {
    const root = tmp();
    mkdirSync(join(root, "fixtures/r/c"), { recursive: true });
    writeFileSync(join(root, "fixtures/r/c/case.yaml"), "id: c\ndiagnostics: [{line: 1, col: 017}]\n");
    const cases = discoverCases(root);
    expect(cases).toHaveLength(1);
    expect(cases[0]?.data).toBeUndefined();
    expect(cases[0]?.parseError).toMatch(/^line 2, col \d+: ambiguous plain scalar "017"/);
  });

  it("surfaces a rules.yaml violation through validateSpec", () => {
    const root = tmp();
    mkdirSync(join(root, "schema"), { recursive: true });
    for (const s of ["rules.schema.json", "fixture.schema.json"]) {
      writeFileSync(join(root, "schema", s), readFileSync(join(STUB_SPEC_ROOT, "schema", s)));
    }
    mkdirSync(join(root, "fixtures"), { recursive: true });
    writeFileSync(join(root, "rules.yaml"), "version: 1\nrules: []\nskipCategories: {}\nx: yes\n");
    const errors = validateSpec(root).errors;
    expect(errors.filter((e) => e.startsWith("rules.yaml: line 4, col 4: ambiguous plain scalar"))).toHaveLength(1);
  });
});

describe("real spec files are unchanged by the subset", () => {
  function yamlFiles(dir: string): string[] {
    const out: string[] = [];
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const p = join(dir, e.name);
      if (e.isDirectory()) out.push(...yamlFiles(p));
      else if (e.name.endsWith(".yaml")) out.push(p);
    }
    return out;
  }

  const roots = [
    ["spec", REPO_SPEC, (f: string) => !f.includes("/probes/")],
    ["stub", STUB_SPEC_ROOT, () => true],
  ] as const;

  it.each(roots)("%s: every file loads and equals plain yaml.parse", (_name, root, keep) => {
    const files = yamlFiles(root).filter(keep);
    expect(files.length).toBeGreaterThan(0);
    for (const f of files) {
      expect(loadYaml(f), f).toStrictEqual(parse(readFileSync(f, "utf8")));
    }
  });
});
