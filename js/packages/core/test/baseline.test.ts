import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, describe, expect, it } from "vitest";
import { BaselineError, formatBaseline, parseBaseline, readBaseline, writeBaseline } from "../src/index.js";

const A = "0c1ab39ac97767156d8a818304b68367";
const B = "d1a64654f64fa217171b2841c61090fd";
const dir = mkdtempSync(join(tmpdir(), "fldlint-baseline-"));
afterAll(() => rmSync(dir, { recursive: true, force: true }));

describe("formatBaseline / writeBaseline", () => {
  it("sorts, dedupes, two-space indent, trailing newline", () => {
    expect(formatBaseline([B, A, B])).toBe(`{\n  "version": 1,\n  "fingerprints": [\n    "${A}",\n    "${B}"\n  ]\n}\n`);
  });
  it("writes an empty list compactly", () => {
    expect(formatBaseline([])).toBe(`{\n  "version": 1,\n  "fingerprints": []\n}\n`);
  });
  it("is byte-stable across a write/read/write round trip", () => {
    const p = join(dir, "b.json");
    writeBaseline(p, [B, A]);
    const first = readFileSync(p, "utf8");
    writeBaseline(p, readBaseline(p));
    expect(readFileSync(p, "utf8")).toBe(first);
  });
  it("rejects invalid fingerprints and unwritable paths", () => {
    expect(() => formatBaseline(["XYZ"])).toThrow(BaselineError);
    expect(() => writeBaseline(join(dir, "no", "such", "b.json"), [A])).toThrow(BaselineError);
  });
});

describe("parseBaseline / readBaseline negative cases", () => {
  const cases: [string, string][] = [
    ["malformed JSON", "{"],
    ["not an object", "[]"],
    ["unknown field", `{"version":1,"fingerprints":[],"extra":1}`],
    ["flaglint format", `{"version":"1","createdAt":"2026-01-01","fingerprints":[]}`],
    ["version 2", `{"version":2,"fingerprints":[]}`],
    ["string version", `{"version":"1","fingerprints":[]}`],
    ["missing fingerprints", `{"version":1}`],
    ["missing version", `{"fingerprints":[]}`],
    ["fingerprints not an array", `{"version":1,"fingerprints":"${A}"}`],
    ["non-hex entry", `{"version":1,"fingerprints":["zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"]}`],
    ["uppercase entry", `{"version":1,"fingerprints":["${A.toUpperCase()}"]}`],
    ["short entry", `{"version":1,"fingerprints":["abc"]}`],
    ["non-string entry", `{"version":1,"fingerprints":[1]}`],
  ];
  it.each(cases)("rejects %s", (_n, text) => {
    expect(() => parseBaseline(text)).toThrow(BaselineError);
    const p = join(dir, "bad.json");
    writeFileSync(p, text);
    expect(() => readBaseline(p)).toThrow(BaselineError);
  });
  it("rejects a missing file", () => expect(() => readBaseline(join(dir, "missing.json"))).toThrow(BaselineError));
  it("accepts a valid file, ignoring order and duplicates", () => {
    expect(parseBaseline(`{"version":1,"fingerprints":["${B}","${A}","${B}"]}`)).toEqual(new Set([A, B]));
  });
});
