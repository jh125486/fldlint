import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { applyEdits, nonOverlapping } from "../src/index.js";

describe("applyEdits", () => {
  it("applies out-of-order edits, UTF-16 ranges", () => {
    expect(applyEdits("😀abc", [{ range: [4, 5], text: "X" }, { range: [2, 3], text: "Y" }])).toBe("😀YbX");
  });
  it("rejects overlap and out-of-range edits", () => {
    expect(() => applyEdits("abcdef", [{ range: [0, 3], text: "" }, { range: [2, 4], text: "" }])).toThrow(RangeError);
    expect(() => applyEdits("abc", [{ range: [2, 9], text: "" }])).toThrow(RangeError);
  });
  it("nonOverlapping keeps the outer edit", () => {
    expect(nonOverlapping([{ range: [2, 3], text: "i" }, { range: [0, 9], text: "o" }])).toEqual([{ range: [0, 9], text: "o" }]);
  });
  it("matches a naive reference on random non-overlapping edits", () => {
    fc.assert(
      fc.property(fc.string(), fc.array(fc.tuple(fc.nat(30), fc.nat(5), fc.string()), { maxLength: 6 }), (src, raw) => {
        const edits = nonOverlapping(
          raw.map(([s, len, text]) => ({ range: [Math.min(s, src.length), Math.min(s + len, src.length)] as [number, number], text })),
        );
        let expected = "";
        let pos = 0;
        for (const e of edits) {
          expected += src.slice(pos, e.range[0]) + e.text;
          pos = e.range[1];
        }
        expected += src.slice(pos);
        expect(applyEdits(src, edits)).toBe(expected);
      }),
    );
  });
});
