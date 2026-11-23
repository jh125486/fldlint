import { Composer, isNode, isScalar, isSeq, LineCounter, Parser } from "yaml";
import type { Pair, Range, Scalar, YAMLError, YAMLMap, YAMLSeq } from "yaml";

/**
 * The "fldlint YAML subset" (spec/contracts/validation-parity.md, section
 * "fldlint YAML subset"). The subset is enforced on the parser's syntax tree
 * before any type resolution: the `yaml` package runs in failsafe mode (every
 * scalar is a string) and the scalar typing below is done here, never by the
 * library.
 */

// Scalar typing rules, named after the contract's labels. Only `^`, `$` and
// `[0-9]` are used (never `\d`) and no lookaround, so they mean the same in
// Go RE2 and JavaScript.
const R1 = /^(true|false)$/;
const R2 = /^(0|-?[1-9][0-9]*)$/;
const R3 = /^(|~|null|Null|NULL)$/;
const R4a = /^(true|false|null|y|n|yes|no|on|off|=|<<)$/i;
const R4b = /^[-+]?([0-9][0-9_,]*(\.[0-9_]*)?|\.[0-9][0-9_]*)([eE][-+]?[0-9]+)?$/;
const R4c = /^[-+]?\.(inf|nan)$/i;
const R4d = /^[-+]?0[xob][0-9a-f_]+$/i;
const R4e = /^[-+]?[0-9][0-9_]*(:[0-5]?[0-9])+(\.[0-9_]*)?$/;
const R4f = /^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}([Tt ]|$)/;

const R4_RULES = [
  ["R4a", R4a],
  ["R4b", R4b],
  ["R4c", R4c],
  ["R4d", R4d],
  ["R4e", R4e],
  ["R4f", R4f],
] as const;

export type AmbiguousRule = (typeof R4_RULES)[number][0];

/** Deepest nesting of collections the subset accepts (contract: 100); the root collection is level 1. */
export const MAX_DEPTH = 100;

const MAX_SAFE = BigInt(Number.MAX_SAFE_INTEGER);

export type PlainClass =
  | { kind: "boolean"; value: boolean }
  | { kind: "integer"; value: number }
  | { kind: "null" }
  | { kind: "string"; value: string }
  /** R4: the scalar would be typed differently by some YAML parser. */
  | { kind: "ambiguous"; rule: AmbiguousRule }
  /** R2: an integer whose absolute value exceeds 2^53 - 1. */
  | { kind: "out-of-range" };

/** Classify the text of a plain scalar (already trimmed and folded) by R1..R5; first match wins. */
export function classifyPlain(text: string): PlainClass {
  if (R1.test(text)) return { kind: "boolean", value: text === "true" };
  if (R2.test(text)) {
    // BigInt, so an arbitrarily long digit string is compared exactly.
    const n = BigInt(text);
    if (n > MAX_SAFE || n < -MAX_SAFE) return { kind: "out-of-range" };
    return { kind: "integer", value: Number(n) };
  }
  if (R3.test(text)) return { kind: "null" };
  for (const [rule, re] of R4_RULES) {
    if (re.test(text)) return { kind: "ambiguous", rule };
  }
  return { kind: "string", value: text };
}

/** Stable machine-readable rejection reasons. */
export type YamlSubsetErrorCode =
  | "encoding"
  | "character"
  | "tab-indent"
  | "empty-document"
  | "directive"
  | "syntax"
  | "multiple-documents"
  | "tag"
  | "anchor"
  | "alias"
  | "complex-key"
  | "key-not-string"
  | "duplicate-key"
  | "ambiguous-scalar"
  | "block-indicator"
  | "flow-scalar"
  | "too-deep"
  | "integer-out-of-range";

/**
 * A YAML subset violation. `line` and `column` are 1-based; the column counts
 * UTF-16 code units (JS string indices, as the project's position contract
 * does) and a leading BOM is not counted.
 */
export class YamlSubsetError extends Error {
  readonly code: YamlSubsetErrorCode;
  readonly line: number;
  readonly column: number;
  readonly reason: string;

  constructor(code: YamlSubsetErrorCode, line: number, column: number, reason: string) {
    super(`line ${String(line)}, col ${String(column)}: ${reason}`);
    this.name = "YamlSubsetError";
    this.code = code;
    this.line = line;
    this.column = column;
    this.reason = reason;
  }
}

const BOM = "﻿";

/**
 * Decode the bytes of a spec file. The subset requires valid UTF-8, so
 * malformed input is rejected rather than repaired; a leading BOM is kept
 * for `parseSubset` to strip.
 *
 * @throws YamlSubsetError with code "encoding".
 */
export function decodeUtf8(bytes: Uint8Array): string {
  try {
    return new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes);
  } catch {
    throw new YamlSubsetError("encoding", 1, 1, "input is not valid UTF-8");
  }
}

/** 1-based line and column (UTF-16 units) of an offset; lines end at LF only (CR, if any, is part of the line). */
function positionOf(text: string, offset: number): { line: number; col: number } {
  const before = text.slice(0, offset);
  return { line: before.split("\n").length, col: offset - before.lastIndexOf("\n") };
}

/** A line is blank when it holds only spaces and tabs, or is a comment: `#` is its first non-space character. */
const BLANK_OR_COMMENT = /^[ \t]*(#.*)?$/;

/**
 * Characters the subset does not allow: anything outside YAML's printable set
 * (tab, LF, CR, U+0020-007E, U+00A0-D7FF, U+E000-FFFD, U+10000-10FFFF; this
 * also catches lone surrogates and NEL), U+FEFF (a BOM is only stripped at the very start), a CR that does not start a CRLF, and
 * the line breaks LS and PS (contract, "Input text", Characters). The `u` flag is for the astral range
 * only; the scalar classifier's regexes do not use it.
 */
const FORBIDDEN = /[^\t\n\r\x20-\x7e\xa0-\ud7ff\ue000-\ufefe\uff00-\ufffd\u{10000}-\u{10ffff}]|\r(?!\n)|[\u2028\u2029]/u;

/** A colon, after optional white space, directly followed by something other than white space (matched from the end of a plain key). */
const GLUED_COLON = /[ \t]*:[^ \t\r\n]/y;

/** A block scalar header with an indentation indicator (`|2`, `>-1`, `|1+`). */
const BLOCK_HEADER_DIGIT = /^[|>][-+]?[0-9]/;

/**
 * A tab in the leading whitespace of a line, whatever the line holds (contract, "Input text", Tab indentation):
 * after zero or more spaces, or after leading `-` or `?` indicators and
 * spaces, as in `-<tab>x`, which the Go parser rejects.
 */
const TAB_INDENT = /(?:^|\n) *(?:[-?][ ]*)*\t/;

/** Rules 1 to 3 on the raw text, in this order of precedence: forbidden characters, tab indentation, no content or directive. Throws on a violation. */
function checkText(text: string): void {
  const bad = FORBIDDEN.exec(text);
  if (bad) {
    const { line, col } = positionOf(text, bad.index);
    const cp = bad[0].charCodeAt(0).toString(16).toUpperCase().padStart(4, "0");
    throw new YamlSubsetError("character", line, col, `character U+${cp} is not allowed (printable characters, LF and CRLF only)`);
  }
  const tab = TAB_INDENT.exec(text);
  if (tab) {
    const { line, col } = positionOf(text, tab.index + tab[0].length - 1);
    throw new YamlSubsetError("tab-indent", line, col, "a tab in the leading whitespace of a line, or after a leading \"-\" or \"?\", is not allowed");
  }
  for (const [i, line] of text.split(/\r?\n/).entries()) {
    if (BLANK_OR_COMMENT.test(line)) continue;
    if (line.startsWith("%")) throw new YamlSubsetError("directive", i + 1, 1, "directives (%...) are not allowed");
    return;
  }
  throw new YamlSubsetError("empty-document", 1, 1, "empty document: no content");
}

interface Property {
  type: "anchor" | "tag" | "alias";
  offset: number;
}

/**
 * The first anchor, tag or alias token in the concrete syntax tree, if any. The
 * composer drops some of them (`{ 'q'\n &a}`), so the tokens, not the nodes,
 * are the source of truth.
 */
function findProperty(tokens: readonly unknown[]): Property | undefined {
  let first: Property | undefined;
  const stack: unknown[] = [...tokens];
  while (stack.length > 0) {
    const v = stack.pop();
    if (typeof v !== "object" || v === null) continue;
    const t = v as { type?: string; offset: number };
    if ((t.type === "anchor" || t.type === "tag" || t.type === "alias") && t.offset < (first?.offset ?? Infinity)) first = t as Property;
    for (const child of Object.values(v)) stack.push(child);
  }
  return first;
}

function errorMessage(e: YAMLError): string {
  // The library appends " at line L, column C:" and a source excerpt; the position is ours to report.
  return e.message.replace(/\n[^]*$/, "").replace(/ at line [0-9]+, column [0-9]+:?$/, "");
}

function plainText(node: Scalar): string {
  // Failsafe mode resolves every scalar to a string (the tests assert this); the typings only know `unknown`.
  return String(node.value);
}

/**
 * Offset of the first character of a parsed node. Nodes produced by the
 * composer always carry a range, but the typings make it optional.
 */
function startOf(node: unknown): number {
  return (node as { range: Range }).range[0];
}

/** Offset just after a parsed node (`range[1]`; see startOf). */
function endOf(node: unknown): number {
  return (node as { range: Range }).range[1];
}

/**
 * Parse `text` as one document of the fldlint YAML subset and return plain
 * data (objects, arrays, strings, safe integers, booleans, null).
 *
 * Input with no document (empty, blank, BOM only or comments only) is an
 * "empty-document" error; a bare `---` is one document whose content is null
 * and returns `null`.
 *
 * @throws YamlSubsetError on any violation of the subset or any YAML error.
 */
export function parseSubset(text: string): unknown {
  const stripped = text.startsWith(BOM) ? text.slice(BOM.length) : text;
  const lineCounter = new LineCounter();
  const fail = (code: YamlSubsetErrorCode, offset: number, reason: string): never => {
    const { line, col } = lineCounter.linePos(offset);
    throw new YamlSubsetError(code, line, col, reason);
  };

  checkText(stripped);
  // A missing final line break is treated as present: the parsers differ on whether a block scalar keeps one.
  const src = stripped.endsWith("\n") ? stripped : `${stripped}\n`;

  const tokens = [...new Parser(lineCounter.addNewLine).parse(src)];
  const docs = [...new Composer({ schema: "failsafe", version: "1.2", uniqueKeys: false, lineCounter }).compose(tokens)];
  for (const doc of docs) {
    const err = doc.errors[0];
    if (err) fail("syntax", err.pos[0], errorMessage(err));
  }
  const [doc, second] = docs;
  if (second) fail("multiple-documents", second.range[0], "more than one document");
  // Zero documents cannot happen after checkText; a lone `...` parses as an empty scalar without a `---`.
  if (!doc || (!doc.directives.docStart && isScalar(doc.contents) && doc.contents.type === "PLAIN" && doc.contents.source === "")) {
    return fail("empty-document", 0, "empty document: no content");
  }
  const property = findProperty(tokens);
  if (property) fail(property.type, property.offset, `${property.type === "tag" ? "explicit tags" : `${property.type}s`} are not allowed`);

  const scalarValue = (node: Scalar): string | number | boolean | null => {
    const text = plainText(node);
    if ((node.type === "BLOCK_LITERAL" || node.type === "BLOCK_FOLDED") && BLOCK_HEADER_DIGIT.test(src.slice(startOf(node), startOf(node) + 4))) {
      fail("block-indicator", startOf(node), "a block scalar indentation indicator (|2, >1-) is not allowed");
    }
    if (node.type !== "PLAIN") return text;
    const c = classifyPlain(text);
    switch (c.kind) {
      case "boolean":
      case "integer":
      case "string":
        return c.value;
      case "null":
        return null;
      case "ambiguous":
        return fail("ambiguous-scalar", startOf(node), `ambiguous plain scalar ${JSON.stringify(text)} (${c.rule}); quote it`);
      case "out-of-range":
        return fail("integer-out-of-range", startOf(node), `integer ${text} is outside the safe range; quote it`);
      // No default: with every PlainClass kind handled the function's return type is satisfied,
      // and adding a kind without handling it here is a compile error (missing return).
    }
  };

  /** A key is a scalar whose classified type is string. */
  const keyOf = (key: unknown): string => {
    if (!isScalar(key)) return fail("complex-key", startOf(key), "mapping keys must be scalars");
    const v = scalarValue(key);
    if (typeof v !== "string") return fail("key-not-string", startOf(key), `mapping key ${JSON.stringify(plainText(key))} is not a string; quote it`);
    return v;
  };

  const convertPairs = (pairs: readonly Pair[], inFlow: boolean, depth: number): Record<string, unknown> => {
    const out: Record<string, unknown> = {};
    for (const pair of pairs) {
      const key = keyOf(pair.key);
      // `[a:]`, `{a:[1]}`: the `yaml` package reads a pair, yaml.v3 the string "a:" or a syntax error; a quoted key may be followed directly by its value, as in JSON.
      if (inFlow && isScalar(pair.key) && pair.key.type === "PLAIN") {
        GLUED_COLON.lastIndex = endOf(pair.key);
        if (GLUED_COLON.test(src)) fail("flow-scalar", startOf(pair.key), "a plain key in a flow collection must be followed by a colon and white space; quote it");
      }
      if (Object.hasOwn(out, key)) fail("duplicate-key", startOf(pair.key), `duplicate key ${JSON.stringify(key)}`);
      // defineProperty, not assignment: `__proto__` must become an ordinary own property.
      Object.defineProperty(out, key, { value: convert(pair.value, inFlow, depth), enumerable: true, writable: true, configurable: true });
    }
    return out;
  };

  const convert = (node: unknown, inFlow: boolean, depth: number): unknown => {
    // An explicit key without a value (`? a`) has a null value node.
    if (!isNode(node)) return null;
    if (isScalar(node)) return scalarValue(node);
    if (depth >= MAX_DEPTH) return fail("too-deep", startOf(node), `nesting is deeper than ${String(MAX_DEPTH)} levels`);
    // A pair `a: b` in a flow sequence is a map without brackets of its own, still in flow context.
    // Only sequences and maps remain: anchors, tags and aliases are rejected above.
    const collection = node as YAMLMap | YAMLSeq;
    const flow = inFlow || collection.flow === true;
    if (isSeq(collection)) return collection.items.map((item) => convert(item, flow, depth + 1));
    return convertPairs(collection.items, flow, depth + 1);
  };

  return convert(doc.contents, false, 0);
}
