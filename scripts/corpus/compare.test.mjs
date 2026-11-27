import test from "node:test";
import assert from "node:assert/strict";
import { compare } from "./compare.mjs";
import { childEnv } from "./run.mjs";

const counts = (o = {}) => ({ total: 0, rewrites: {}, skips: {}, noFix: {}, ...o });
const okBuild = { cmd: "go build ./...", ok: true, exit: 0 };
const badBuild = { cmd: "go build ./...", ok: false, exit: 1 };
const repo = (o = {}) => ({
  status: "ok",
  sha: "a".repeat(40),
  counts: counts({ total: 2, skips: { "dynamic-key": 2 } }),
  build: { pre: okBuild, post: { skipped: "nothing changed" } },
  ...o,
});
const snap = { repos: { a: repo() } };

test("compare", async (t) => {
  const cases = [
    { name: "identical result passes", result: { a: repo() }, fail: 0, warn: 0 },
    { name: "tool crash fails", result: { a: repo({ status: "error", errorKind: "tool", error: "boom" }) }, fail: 2, warn: 0 },
    { name: "error without a kind counts as a tool error", result: { a: repo({ status: "error", error: "boom" }) }, fail: 2, warn: 0 },
    { name: "infra error only warns, but then nothing ran ok", result: { a: repo({ status: "error", errorKind: "infra", error: "clone" }) }, fail: 1, warn: 1 },
    { name: "infra error next to an ok repo warns", result: { a: repo(), b: repo({ status: "error", errorKind: "infra", error: "clone" }) }, fail: 0, warn: 1 },
    { name: "timeout warns", result: { a: repo(), b: repo({ status: "timeout", error: "t" }) }, fail: 0, warn: 1 },
    { name: "every repo errored fails", result: { a: repo({ status: "error", errorKind: "infra", error: "x" }), b: repo({ status: "timeout", error: "t" }) }, fail: 1, warn: 2 },
    { name: "empty result fails", result: {}, fail: 1, warn: 0 },
    { name: "rewrite rise fails", result: { a: repo({ counts: counts({ total: 3, rewrites: { "bool-variation": 1 }, skips: { "dynamic-key": 2 } }) }) }, fail: 1, warn: 0 },
    { name: "new repo with rewrites fails", result: { a: repo(), n: repo({ counts: counts({ total: 1, rewrites: { "bool-variation": 1 } }) }) }, fail: 1, warn: 0 },
    { name: "new repo without rewrites passes", result: { a: repo(), n: repo() }, fail: 0, warn: 0 },
    { name: "post-fix build break fails", result: { a: repo({ build: { pre: okBuild, post: badBuild } }) }, fail: 1, warn: 0 },
    { name: "pre-fix build already broken warns", result: { a: repo({ build: { pre: badBuild, post: badBuild } }) }, fail: 0, warn: 1 },
    { name: "post-fix build timeout warns", result: { a: repo({ build: { pre: okBuild, post: { ...badBuild, timedOut: true } } }) }, fail: 0, warn: 1 },
    { name: "total drop to zero fails", result: { a: repo({ counts: counts() }) }, fail: 1, warn: 0 },
    { name: "total drop to non-zero is a note", result: { a: repo({ counts: counts({ total: 1, skips: { "dynamic-key": 1 } }) }) }, fail: 0, warn: 0 },
    { name: "no-fix rise fails (rewrite moved into noFix)", result: { a: repo({ counts: counts({ total: 2, skips: { "dynamic-key": 1 }, noFix: { "bool-variation:comment": 1 } }) }) }, fail: 1, warn: 0 },
    { name: "analysis errors on a building repo fail", result: { a: repo({ counts: counts({ total: 2, skips: { "dynamic-key": 2 }, analysisErrors: { pkg: 1 } }) }) }, fail: 1, warn: 0 },
    { name: "analysis errors on a broken repo warn", result: { a: repo({ build: { pre: badBuild, post: { skipped: "x" } }, counts: counts({ total: 2, skips: { "dynamic-key": 2 }, analysisErrors: { pkg: 1 } }) }) }, fail: 0, warn: 1 },
  ];
  for (const c of cases) {
    await t.test(c.name, () => {
      const r = compare({ repos: c.result }, snap);
      assert.equal(r.failures.length, c.fail, JSON.stringify(r));
      assert.equal(r.warnings.length, c.warn, JSON.stringify(r));
    });
  }
});

test("childEnv", async (t) => {
  const base = { PATH: "/bin", HOME: "/h", GH_TOKEN: "x", AWS_SECRET_ACCESS_KEY: "y", SSH_AUTH_SOCK: "/s", NPM_TOKEN: "z", NODE_AUTH_TOKEN: "w" };
  await t.test("drops secrets for both profiles", () => {
    for (const trusted of [true, false]) {
      const env = childEnv(trusted, {}, base);
      for (const k of ["GH_TOKEN", "AWS_SECRET_ACCESS_KEY", "SSH_AUTH_SOCK", "NPM_TOKEN", "NODE_AUTH_TOKEN"]) assert.equal(env[k], undefined, k);
      assert.equal(env.PATH, "/bin");
      assert.equal(env.GIT_CONFIG_GLOBAL, "/dev/null");
    }
  });
  await t.test("untrusted pins npm registry and disables scripts; trusted keeps scripts enabled", () => {
    const u = childEnv(false, {}, base);
    assert.equal(u.NPM_CONFIG_REGISTRY, "https://registry.npmjs.org/");
    assert.equal(u.NPM_CONFIG_IGNORE_SCRIPTS, "true");
    const t2 = childEnv(true, {}, base);
    assert.equal(t2.NPM_CONFIG_IGNORE_SCRIPTS, undefined);
  });
});
