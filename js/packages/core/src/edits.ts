import type { Edit } from "@fldlint/spec";

/** Sort edits by start and drop any that overlap an earlier (outer or leftmost) one. */
export function nonOverlapping(edits: readonly Edit[]): Edit[] {
  const sorted = [...edits].sort((a, b) => a.range[0] - b.range[0] || b.range[1] - a.range[1]);
  const kept: Edit[] = [];
  let end = 0;
  for (const e of sorted) {
    if (e.range[0] < end) continue;
    kept.push(e);
    end = e.range[1];
  }
  return kept;
}

/**
 * Apply edits (half-open UTF-16 ranges) to `source`. Edits may be given in any
 * order but must not overlap; use {@link nonOverlapping} first when they might.
 */
export function applyEdits(source: string, edits: readonly Edit[]): string {
  const sorted = [...edits].sort((a, b) => a.range[0] - b.range[0] || a.range[1] - b.range[1]);
  let out = "";
  let pos = 0;
  for (const { range: [start, end], text } of sorted) {
    if (start < pos) throw new RangeError(`overlapping edits at ${start}`);
    if (start > end || end > source.length) throw new RangeError(`edit range [${start}, ${end}) out of bounds`);
    out += source.slice(pos, start) + text;
    pos = end;
  }
  return out + source.slice(pos);
}
