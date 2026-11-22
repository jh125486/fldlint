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
  it("rejects negative, fractional and surrogate-splitting ranges with a specific message", () => {
    expect(() => applyEdits("abc", [{ range: [-1, 1], text: "" }])).toThrow(/non-negative integers/);
    expect(() => applyEdits("abc", [{ range: [0.5, 1], text: "" }])).toThrow(/non-negative integers/);
    expect(() => applyEdits("abc", [{ range: [0, Number.NaN], text: "" }])).toThrow(/non-negative integers/);
    expect(() => applyEdits("abc", [{ range: [2, 1], text: "" }])).toThrow(/reversed/);
    expect(() => applyEdits("abc", [{ range: [0, 4], text: "" }])).toThrow(/out of bounds/);
    expect(() => applyEdits("a😀b", [{ range: [2, 3], text: "" }])).toThrow(/surrogate/);
    expect(() => applyEdits("a😀b", [{ range: [1, 2], text: "" }])).toThrow(/surrogate/);
    expect(applyEdits("a😀b", [{ range: [1, 3], text: "" }])).toBe("ab");
    expect(applyEdits("a\ud83d", [{ range: [1, 2], text: "" }])).toBe("a"); // lone surrogate is fine
  });

  // The reference is built from the cut points, independent of nonOverlapping/applyEdits ordering.
  const isSplit = (src: string, i: number) => /[\udc00-\udfff]/.test(src[i] ?? "") && /[\ud800-\udbff]/.test(src[i - 1] ?? "");
  const arbPlan = fc
    .tuple(fc.string({ maxLength: 40 }), fc.array(fc.tuple(fc.nat(), fc.nat(), fc.string({ maxLength: 4 })), { maxLength: 6 }))
    .map(([src, raw]) => {
      const cuts = raw.flatMap(([a, b]) => [a % (src.length + 1), b % (src.length + 1)]).sort((x, y) => x - y);
      const edits: { range: [number, number]; text: string }[] = [];
      for (let i = 0; i + 1 < cuts.length; i += 2) edits.push({ range: [cuts[i]!, cuts[i + 1]!], text: raw[i / 2]![2] });
      return { src, edits };
    })
    // Several inserts at one offset have no defined relative order; keep starts unique.
    .filter(({ src, edits }) => new Set(edits.map((e) => e.range[0])).size === edits.length && edits.every((e) => e.range.every((i) => !isSplit(src, i))));

  it("applies disjoint edits in any order like a segment-wise reference", () => {
    fc.assert(
      fc.property(arbPlan, fc.nat(), ({ src, edits }, seed) => {
        const reference = edits.reduceRight((acc, e) => acc.slice(0, e.range[0]) + e.text + acc.slice(e.range[1]), src);
        const rotated = edits.map((_, i) => edits[(i + seed) % edits.length]!);
        expect(applyEdits(src, rotated)).toBe(reference);
      }),
    );
  });

  it("nonOverlapping returns sorted disjoint edits, and every dropped edit overlaps a kept one", () => {
    fc.assert(
      fc.property(fc.array(fc.tuple(fc.nat(30), fc.nat(6), fc.string({ maxLength: 3 })), { maxLength: 8 }), (raw) => {
        const input = raw.map(([s, len, text]) => ({ range: [s, s + len] as [number, number], text }));
        const kept = nonOverlapping(input);
        for (let i = 1; i < kept.length; i++) expect(kept[i]!.range[0]).toBeGreaterThanOrEqual(kept[i - 1]!.range[1]);
        for (const e of input) {
          if (kept.includes(e)) continue;
          expect(kept.some((k) => e.range[0] < k.range[1] && k.range[0] < Math.max(e.range[1], e.range[0] + 1))).toBe(true);
        }
      }),
    );
  });
});
