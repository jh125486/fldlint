import { createHash } from "node:crypto";

/** The identity inputs of a finding (spec/contracts/fingerprint.md). */
export interface FingerprintInput {
  ruleId: string;
  /** Path relative to the scan root; normalized here. */
  path: string;
  /** Decoded string value of a static flag key, or null when there is none. */
  key: string | null;
  /** 0-based ordinal among same (ruleId, kind, key) findings in the file. */
  occ: number;
}

/**
 * Normalize a path: `\` to `/`, collapse `/` runs, strip leading `./` (repeatedly)
 * and interior `/./`, strip trailing `/`. No case folding or Unicode normalization.
 */
export function normalizePath(p: string): string {
  let s = p.replaceAll("\\", "/").replace(/\/{2,}/g, "/");
  while (s.startsWith("./")) s = s.slice(2);
  while (s.includes("/./")) s = s.replaceAll("/./", "/");
  return s.replace(/\/+$/, "");
}

/** 32 lowercase hex chars: SHA-256 of `ruleId NUL path NUL kind NUL key NUL occ` (UTF-8). */
export function fingerprint(f: FingerprintInput): string {
  const kind = f.key === null ? "-" : "s";
  const preimage = [f.ruleId, normalizePath(f.path), kind, f.key ?? "", String(f.occ)].join("\0");
  return createHash("sha256").update(preimage, "utf8").digest("hex").slice(0, 32);
}

export interface Positioned {
  ruleId: string;
  key: string | null;
  line: number;
  col: number;
}

/**
 * Assign `occ` to the findings of ONE file: the 0-based rank among findings with
 * the same (ruleId, kind, key), ordered by (line, col). Input order is preserved.
 */
export function assignOccurrences<T extends Positioned>(findings: readonly T[]): (T & { occ: number })[] {
  const groups = new Map<string, number[]>();
  findings.forEach((f, i) => {
    const id = JSON.stringify([f.ruleId, f.key === null ? "-" : "s", f.key ?? ""]);
    const g = groups.get(id);
    if (g) g.push(i);
    else groups.set(id, [i]);
  });
  const occ = new Array<number>(findings.length).fill(0);
  for (const idx of groups.values()) {
    idx.sort((a, b) => findings[a]!.line - findings[b]!.line || findings[a]!.col - findings[b]!.col || a - b);
    idx.forEach((fi, rank) => (occ[fi] = rank));
  }
  return findings.map((f, i) => ({ ...f, occ: occ[i]! }));
}
