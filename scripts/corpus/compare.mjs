// Pass/fail logic of the corpus (pure functions, unit-tested in compare.test.mjs).
// Rules are documented in scripts/corpus/README.md ("Pass / fail").

export const sum = (o) => Object.values(o ?? {}).reduce((a, b) => a + b, 0);

/**
 * @param {{repos: Record<string, any>}} result  a run's result.json
 * @param {{repos: Record<string, any>} | null} snapshot  the committed snapshot
 * @returns {{failures: string[], warnings: string[], notes: string[]}}
 */
export function compare(result, snapshot) {
  const failures = [];
  const warnings = [];
  const notes = [];
  const entries = Object.entries(result?.repos ?? {});
  if (entries.length === 0) failures.push("the result has no repositories");
  else if (!entries.some(([, r]) => r.status === "ok")) failures.push("no repository ran ok (nothing was verified)");

  for (const [name, r] of entries) {
    const s = snapshot?.repos?.[name];
    if (r.status !== "ok") {
      // "tool" errors are fldlint (or this runner) misbehaving and fail; "infra" errors (clone, fetch,
      // dependencies, timeouts) are the network or the third-party repo and only warn.
      const kind = r.status === "timeout" ? "infra" : (r.errorKind ?? "tool");
      (kind === "tool" ? failures : warnings).push(`${name}: ${r.status} (${kind}): ${r.error}`);
      continue;
    }
    const pre = r.build?.pre;
    const post = r.build?.post;
    if (post && post.ok === false) {
      if (post.timedOut) warnings.push(`${name}: ${post.cmd} timed out after the fixes were applied; no safety verdict`);
      else if (pre && pre.ok === false) warnings.push(`${name}: the build already fails before fixing (${pre.cmd}), so no safety verdict`);
      else failures.push(`${name}: SAFETY BUG: ${post.cmd} fails after the fixes were applied (it passed before)`);
    }
    const counts = r.counts ?? { total: 0, rewrites: {}, skips: {}, noFix: {} };
    if (counts.analysisErrors) {
      const msg = `${name}: the analysis reported errors for ${Object.keys(counts.analysisErrors).length} package(s)`;
      if (pre && pre.ok === true) failures.push(`${msg} although the repo builds`);
      else warnings.push(msg);
    }
    if (counts.parseErrors) notes.push(`${name}: ${sum(counts.parseErrors)} file(s) failed to parse`);

    if (!s || s.status !== "ok") {
      if (!s) notes.push(`${name}: not in the snapshot; run \`task corpus:update\` after review`);
      if (!s && sum(counts.rewrites) > 0) failures.push(`${name}: new repo with ${sum(counts.rewrites)} rewrite(s) and no snapshot entry`);
      continue;
    }
    if (r.sha !== s.sha) notes.push(`${name}: pinned SHA changed (${s.sha.slice(0, 12)} -> ${r.sha.slice(0, 12)})`);
    if (s.counts.total > 0 && counts.total === 0) {
      failures.push(`${name}: findings dropped from ${s.counts.total} to 0 (the tool may have silently stopped finding calls)`);
    } else if (counts.total !== s.counts.total) {
      notes.push(`${name}: findings ${s.counts.total} -> ${counts.total}`);
    }
    for (const rule of [...new Set([...Object.keys(counts.rewrites), ...Object.keys(s.counts.rewrites)])].sort()) {
      const now = counts.rewrites[rule] ?? 0;
      const was = s.counts.rewrites[rule] ?? 0;
      if (now > was) failures.push(`${name}: rewrite count for ${rule} rose ${was} -> ${now} (add a fixture, then \`task corpus:update\`)`);
      else if (now < was) notes.push(`${name}: rewrite count for ${rule} fell ${was} -> ${now}`);
    }
    for (const k of [...new Set([...Object.keys(counts.noFix), ...Object.keys(s.counts.noFix)])].sort()) {
      const now = counts.noFix[k] ?? 0;
      const was = s.counts.noFix[k] ?? 0;
      if (now > was) failures.push(`${name}: no-fix count for ${k} rose ${was} -> ${now} (a rewrite may have been withheld)`);
      else if (now < was) notes.push(`${name}: no-fix ${k} ${was} -> ${now}`);
    }
    for (const k of [...new Set([...Object.keys(counts.skips), ...Object.keys(s.counts.skips)])].sort()) {
      const now = counts.skips[k] ?? 0;
      const was = s.counts.skips[k] ?? 0;
      if (now !== was) notes.push(`${name}: skips ${k} ${was} -> ${now}`);
    }
  }
  return { failures, warnings, notes };
}
