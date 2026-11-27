#!/usr/bin/env node
// Real-repo corpus runner (issue #21, Part 2). Read scripts/corpus/README.md.
//
//   node scripts/corpus/run.mjs run     [--lang js|go|java|all] [--only a,b] [--out file]
//   node scripts/corpus/run.mjs compare <result.json> [snapshot.json]
//   node scripts/corpus/run.mjs summary <result.json> [snapshot.json]
//
// Uses only Node built-ins plus the `yaml` package that js/ already depends on.
// Every repo is cloned into a temp dir, analysed in report mode, fixed in that
// temp copy only, built, and deleted. Nothing is ever pushed or commented.
import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const jsDir = path.join(root, "js");
const requireJs = createRequire(path.join(jsDir, "package.json"));
const MAVEN_PLUGIN = "org.openrewrite.maven:rewrite-maven-plugin:6.46.1";
const RECIPES_ALL =
  "io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature,io.github.jh125486.fldlint.FindLaunchDarklyUsage";
const RECIPE_FIX = "io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature";
const LANGS = ["js", "go", "java"];

// ---------------------------------------------------------------- processes

const children = new Set();

/** Run a command; kills its whole process group at the deadline. Never throws. */
function sh(cmd, args, { cwd, env, deadline, input } = {}) {
  return new Promise((resolve) => {
    const budget = deadline ? deadline - Date.now() : Infinity;
    if (budget <= 0) return resolve({ code: null, stdout: "", stderr: "", timedOut: true });
    const child = spawn(cmd, args, {
      cwd,
      env: { ...process.env, GIT_TERMINAL_PROMPT: "0", CI: "1", ...env },
      detached: true,
      stdio: ["pipe", "pipe", "pipe"],
    });
    children.add(child);
    let stdout = "";
    let stderr = "";
    let timedOut = false;
    child.stdout.on("data", (d) => (stdout += d));
    child.stderr.on("data", (d) => (stderr += d));
    child.stdin.on("error", () => {});
    child.stdin.end(input ?? "");
    const timer = Number.isFinite(budget)
      ? setTimeout(() => {
          timedOut = true;
          kill(child);
        }, budget)
      : null;
    const done = (code) => {
      if (timer) clearTimeout(timer);
      children.delete(child);
      resolve({ code, stdout, stderr, timedOut });
    };
    child.on("error", (e) => {
      stderr += String(e);
      done(127);
    });
    child.on("close", (code) => done(code));
  });
}

function kill(child) {
  try {
    process.kill(-child.pid, "SIGKILL");
  } catch {
    try {
      child.kill("SIGKILL");
    } catch {
      /* already gone */
    }
  }
}

const tail = (s, n = 2000) => (s.length > n ? "..." + s.slice(-n) : s);

class Timeout extends Error {}
class StepError extends Error {}
const must = (r, what) => {
  if (r.timedOut) throw new Timeout(what);
  if (r.code !== 0) throw new StepError(`${what} failed (exit ${r.code}): ${tail(r.stderr || r.stdout, 600)}`);
  return r;
};

// ------------------------------------------------------------------- corpus

function loadCorpus() {
  const { parse } = requireJs("yaml");
  const doc = parse(fs.readFileSync(path.join(root, "spec/corpus.yaml"), "utf8"));
  const repos = [];
  const seen = new Set();
  for (const lang of LANGS) {
    for (const r of doc[lang] ?? []) {
      for (const k of ["name", "url", "sha", "license", "note", "timeout", "depth"]) {
        if (r[k] === undefined || r[k] === "") throw new Error(`corpus.yaml: ${lang}/${r.name}: missing ${k}`);
      }
      if (r.language !== lang) throw new Error(`corpus.yaml: ${r.name}: language ${r.language} under ${lang}`);
      if (!/^[0-9a-f]{40}$/.test(r.sha)) throw new Error(`corpus.yaml: ${r.name}: sha must be 40 hex chars`);
      if (!/^https:\/\/github\.com\//.test(r.url)) throw new Error(`corpus.yaml: ${r.name}: url must be https://github.com/...`);
      if (seen.has(r.name)) throw new Error(`corpus.yaml: duplicate name ${r.name}`);
      seen.add(r.name);
      repos.push(r);
    }
  }
  return repos;
}

// ------------------------------------------------------------------- counts

const SKIP_RE = /\[skip: ([a-z-]+)\]/;
const NOFIX_RE = /\[no-fix: ([a-z-]+)\]/;

function newCounts() {
  return { total: 0, rewrites: {}, skips: {}, noFix: {} };
}
function bump(o, k) {
  o[k] = (o[k] ?? 0) + 1;
}
/** One finding: {rule, skip?, noFix?}. A rewrite is neither skipped nor withheld. */
function addFinding(c, { rule, skip, noFix }) {
  c.total++;
  if (skip) bump(c.skips, skip);
  else if (noFix) bump(c.noFix, `${rule}:${noFix}`);
  else bump(c.rewrites, rule);
}
function fromMessage(rule, message) {
  return { rule, skip: SKIP_RE.exec(message)?.[1], noFix: NOFIX_RE.exec(message)?.[1] };
}
function sortKeys(o) {
  if (Array.isArray(o)) return o.map(sortKeys);
  if (o && typeof o === "object") return Object.fromEntries(Object.keys(o).sort().map((k) => [k, sortKeys(o[k])]));
  return o;
}

// ------------------------------------------------------------------ cloning

async function clone(repo, dir, deadline) {
  must(await sh("git", ["init", "-q", dir], { deadline }), "git init");
  must(await sh("git", ["remote", "add", "origin", repo.url], { cwd: dir, deadline }), "git remote add");
  must(
    await sh("git", ["fetch", "-q", "--depth", String(repo.depth), "origin", repo.sha], { cwd: dir, deadline }),
    "git fetch",
  );
  must(await sh("git", ["checkout", "-q", "FETCH_HEAD"], { cwd: dir, deadline }), "git checkout");
  const head = (await sh("git", ["rev-parse", "HEAD"], { cwd: dir, deadline })).stdout.trim();
  if (head !== repo.sha) throw new StepError(`checked out ${head}, expected ${repo.sha}`);
}

/** Commit the dependency prep so later diffs show only fldlint's own edits. */
async function commitBaseline(dir, deadline) {
  await sh("git", ["add", "-A"], { cwd: dir, deadline });
  await sh(
    "git",
    ["-c", "user.name=corpus", "-c", "user.email=corpus@invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "prep"],
    { cwd: dir, deadline },
  );
}

async function changedFiles(dir, deadline) {
  const r = await sh("git", ["status", "--porcelain", "--untracked-files=no"], { cwd: dir, deadline });
  return r.stdout.split("\n").filter(Boolean).length;
}
async function fixPatch(dir, deadline) {
  return (await sh("git", ["diff"], { cwd: dir, deadline })).stdout;
}

const buildResult = (r, cmd) => ({
  cmd,
  ok: !r.timedOut && r.code === 0,
  ...(r.timedOut ? { timedOut: true } : {}),
  exit: r.code,
  ...(r.code === 0 && !r.timedOut ? {} : { log: tail(r.stderr || r.stdout) }),
});
const skippedBuild = (why) => ({ skipped: why });

// ----------------------------------------------------------------------- JS

async function prepareJs() {
  const r = await sh("npm", ["run", "build", "--silent"], { cwd: jsDir });
  if (r.code !== 0) throw new Error(`building js/ failed: ${tail(r.stderr || r.stdout)}`);
}

function eslintConfig(tmp) {
  const plugin = pathToFileURL(path.join(jsDir, "packages/eslint-plugin/dist/index.js")).href;
  const parser = pathToFileURL(requireJs.resolve("@typescript-eslint/parser")).href;
  const file = path.join(tmp, "eslint.config.mjs");
  fs.writeFileSync(
    file,
    `import fldlint from ${JSON.stringify(plugin)};
import * as tsParser from ${JSON.stringify(parser)};
export default [
  { ignores: ["**/node_modules/**", "**/dist/**", "**/build/**", "**/coverage/**", "**/.next/**"] },
  fldlint.configs.recommended,
  { files: ["**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts}"], languageOptions: { parser: tsParser } },
];
`,
  );
  return file;
}

async function runEslint(dir, cfg, deadline, fix) {
  const eslint = path.join(jsDir, "node_modules/.bin/eslint");
  const args = ["--config", cfg, "--format", "json", "--no-warn-ignored"];
  if (fix) args.push("--fix");
  args.push(".");
  const r = await sh(eslint, args, { cwd: dir, deadline });
  if (r.timedOut) throw new Timeout("eslint");
  if (r.code !== 0 && r.code !== 1) throw new StepError(`eslint exit ${r.code}: ${tail(r.stderr || r.stdout, 600)}`);
  let files;
  try {
    files = JSON.parse(r.stdout);
  } catch {
    throw new StepError(`eslint did not print JSON: ${tail(r.stdout || r.stderr, 600)}`);
  }
  return files;
}

async function tscBuild(dir, deadline) {
  if (!fs.existsSync(path.join(dir, "tsconfig.json"))) return skippedBuild("no tsconfig.json");
  const local = path.join(dir, "node_modules/.bin/tsc");
  const tsc = fs.existsSync(local) ? local : path.join(jsDir, "node_modules/.bin/tsc");
  return buildResult(await sh(tsc, ["--noEmit", "-p", "tsconfig.json"], { cwd: dir, deadline }), "tsc --noEmit");
}

async function analyzeJs(repo, dir, tmp, deadline, out) {
  const cfg = eslintConfig(tmp);
  if (fs.existsSync(path.join(dir, "package.json")) && fs.existsSync(path.join(dir, "tsconfig.json"))) {
    const r = await sh("npm", ["install", "--ignore-scripts", "--no-audit", "--no-fund", "--loglevel=error"], {
      cwd: dir,
      deadline,
    });
    if (r.timedOut) throw new Timeout("npm install");
    out.deps = { ok: r.code === 0, ...(r.code === 0 ? {} : { log: tail(r.stderr) }) };
  }
  await commitBaseline(dir, deadline);
  out.build.pre = await tscBuild(dir, deadline);
  const counts = newCounts();
  for (const f of await runEslint(dir, cfg, deadline, false)) {
    for (const m of f.messages) {
      if (m.ruleId === null && m.fatal) {
        bump((counts.parseErrors ??= {}), "parse-error");
        continue;
      }
      if (m.ruleId?.startsWith("fldlint/")) addFinding(counts, fromMessage(m.ruleId.slice(8), m.message));
    }
  }
  out.counts = counts;
  await runEslint(dir, cfg, deadline, true);
  out.fixedFiles = await changedFiles(dir, deadline);
  out.patch = await fixPatch(dir, deadline);
  out.build.post = out.fixedFiles === 0 ? skippedBuild("nothing changed") : await tscBuild(dir, deadline);
}

// ----------------------------------------------------------------------- Go

let fldlintBin;
async function prepareGo(tmp) {
  fldlintBin = path.join(tmp, "bin", "fldlint");
  const r = await sh("go", ["build", "-o", fldlintBin, "./cmd/fldlint"], { cwd: path.join(root, "go") });
  if (r.code !== 0) throw new Error(`building fldlint failed: ${tail(r.stderr)}`);
}

async function analyzeGo(repo, dir, tmp, deadline, out) {
  const env = {};
  if (!fs.existsSync(path.join(dir, "go.mod"))) throw new StepError("no go.mod at the repository root");
  if (fs.existsSync(path.join(dir, "go.sum"))) {
    must(await sh("go", ["mod", "download"], { cwd: dir, deadline, env }), "go mod download");
  } else {
    // Some samples commit no go.sum; tidy fills it (temp copy only).
    must(await sh("go", ["mod", "tidy"], { cwd: dir, deadline, env }), "go mod tidy");
    out.tidied = true;
  }
  await commitBaseline(dir, deadline);
  out.build.pre = buildResult(await sh("go", ["build", "./..."], { cwd: dir, deadline, env }), "go build ./...");
  const r = await sh(fldlintBin, ["-json", "./..."], { cwd: dir, deadline, env });
  if (r.timedOut) throw new Timeout("fldlint -json");
  if (r.code !== 0) throw new StepError(`fldlint -json exit ${r.code}: ${tail(r.stderr || r.stdout, 600)}`);
  const counts = newCounts();
  const text = r.stdout.trim();
  const doc = text ? JSON.parse(text) : {};
  for (const [pkg, analyzers] of Object.entries(doc)) {
    const res = analyzers.fldlint;
    if (!res) continue;
    if (!Array.isArray(res)) {
      bump((counts.analysisErrors ??= {}), pkg);
      continue;
    }
    for (const d of res) addFinding(counts, fromMessage(d.category, d.message));
  }
  out.counts = counts;
  must(await sh(fldlintBin, ["-fix", "./..."], { cwd: dir, deadline, env }), "fldlint -fix");
  out.fixedFiles = await changedFiles(dir, deadline);
  out.patch = await fixPatch(dir, deadline);
  out.build.post =
    out.fixedFiles === 0
      ? skippedBuild("nothing changed")
      : buildResult(await sh("go", ["build", "./..."], { cwd: dir, deadline, env }), "go build ./...");
}

// --------------------------------------------------------------------- Java

let javaCoords;
async function prepareJava() {
  const javaDir = path.join(root, "java");
  const mvnw = path.join(javaDir, "mvnw");
  let r = await sh(mvnw, ["-B", "-q", "-DskipTests", "install"], { cwd: javaDir });
  if (r.code !== 0) throw new Error(`installing the recipes failed: ${tail(r.stderr || r.stdout)}`);
  r = await sh(
    mvnw,
    ["-q", "-B", "org.apache.maven.plugins:maven-help-plugin:3.6.0:evaluate", "-Dexpression=project.version", "-DforceStdout"],
    { cwd: javaDir },
  );
  if (r.code !== 0) throw new Error(`reading the recipe version failed: ${tail(r.stderr)}`);
  javaCoords = `io.github.jh125486:fldlint-rewrite:${r.stdout.trim()}`;
}

/** Minimal RFC 4180 reader: quoted fields, doubled quotes, embedded newlines. */
function parseCsv(text) {
  const rows = [];
  let row = [];
  let f = "";
  let q = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (q) {
      if (c === '"' && text[i + 1] === '"') (f += '"'), i++;
      else if (c === '"') q = false;
      else f += c;
    } else if (c === '"') q = true;
    else if (c === ",") (row.push(f), (f = ""));
    else if (c === "\n" || c === "\r") {
      if (c === "\r" && text[i + 1] === "\n") i++;
      row.push(f);
      f = "";
      if (row.length > 1 || row[0] !== "") rows.push(row);
      row = [];
    } else f += c;
  }
  if (f !== "" || row.length) (row.push(f), rows.push(row));
  return rows;
}

function findFiles(dir, name, acc = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) findFiles(p, name, acc);
    else if (e.name.endsWith(name)) acc.push(p);
  }
  return acc;
}

async function analyzeJava(repo, dir, tmp, deadline, out) {
  const mvnw = path.join(root, "java", "mvnw");
  if (!fs.existsSync(path.join(dir, "pom.xml"))) throw new StepError("no pom.xml at the repository root");
  const common = ["-B", "-Drewrite.recipeArtifactCoordinates=" + javaCoords];
  out.build.pre = buildResult(await sh(mvnw, ["-B", "-q", "compile"], { cwd: dir, deadline }), "mvn -q compile");
  must(
    await sh(
      mvnw,
      [...common, `${MAVEN_PLUGIN}:dryRun`, `-Drewrite.activeRecipes=${RECIPES_ALL}`, "-Drewrite.exportDatatables=true"],
      { cwd: dir, deadline },
    ),
    "rewrite:dryRun",
  );
  const counts = newCounts();
  const csvs = findFiles(path.join(dir, "target", "rewrite"), "LaunchDarklyUsage.csv");
  if (csvs.length === 0 && fs.existsSync(path.join(dir, "target", "rewrite", "datatables"))) {
    throw new StepError("dryRun exported no LaunchDarklyUsage data table");
  }
  const seen = new Set();
  for (const csv of csvs) {
    // OpenRewrite prefixes the CSV with `# @name ...` comment lines.
    const [head, ...rows] = parseCsv(fs.readFileSync(csv, "utf8")).filter((r) => !r[0].startsWith("#"));
    const col = (n) => head.indexOf(n);
    const [iRule, iSkip, iNoFix, iFp] = [col("ruleId"), col("skip"), col("noFix"), col("fingerprint")];
    if (iRule < 0 || iSkip < 0) throw new StepError(`unexpected data table columns: ${head.join("|")}`);
    for (const r of rows) {
      const key = `${r[col("sourcePath")]}:${r[col("line")]}:${r[col("col")]}:${iFp >= 0 ? r[iFp] : ""}`;
      if (seen.has(key)) continue;
      seen.add(key);
      addFinding(counts, { rule: r[iRule], skip: r[iSkip] || undefined, noFix: (iNoFix >= 0 && r[iNoFix]) || undefined });
    }
  }
  out.counts = counts;
  must(
    await sh(mvnw, [...common, `${MAVEN_PLUGIN}:run`, `-Drewrite.activeRecipes=${RECIPE_FIX}`], { cwd: dir, deadline }),
    "rewrite:run",
  );
  out.fixedFiles = await changedFiles(dir, deadline);
  out.patch = await fixPatch(dir, deadline);
  out.build.post =
    out.fixedFiles === 0
      ? skippedBuild("nothing changed")
      : buildResult(await sh(mvnw, ["-B", "-q", "compile"], { cwd: dir, deadline }), "mvn -q compile");
}

// ------------------------------------------------------------------- driver

const analyzers = { js: analyzeJs, go: analyzeGo, java: analyzeJava };

async function runRepo(repo, outDir) {
  const started = Date.now();
  const deadline = started + repo.timeout * 1000;
  const tmp = fs.mkdtempSync(path.join(process.env.RUNNER_TEMP || os.tmpdir(), `fldlint-corpus-${repo.name}-`));
  const dir = path.join(tmp, "repo");
  const out = {
    name: repo.name,
    language: repo.language,
    url: repo.url,
    sha: repo.sha,
    license: repo.license,
    status: "ok",
    build: {},
  };
  try {
    await clone(repo, dir, deadline);
    await analyzers[repo.language](repo, dir, tmp, deadline, out);
  } catch (e) {
    out.status = e instanceof Timeout ? "timeout" : "error";
    out.error = e instanceof Timeout ? `timed out after ${repo.timeout}s in: ${e.message}` : tail(String(e.message ?? e), 1500);
  } finally {
    if (out.patch && outDir && (out.build.post?.ok === false || process.env.CORPUS_KEEP_PATCHES === "1")) {
      fs.mkdirSync(outDir, { recursive: true });
      fs.writeFileSync(path.join(outDir, `${repo.name}.fix.patch`), out.patch);
    }
    delete out.patch;
    fs.rmSync(tmp, { recursive: true, force: true });
    out.durationMs = Date.now() - started;
  }
  return out;
}

function arg(argv, name, dflt) {
  const i = argv.indexOf(name);
  return i >= 0 ? argv[i + 1] : dflt;
}

async function cmdRun(argv) {
  const lang = arg(argv, "--lang", "all");
  const only = arg(argv, "--only", "")?.split(",").filter(Boolean) ?? [];
  const outFile = path.resolve(arg(argv, "--out", path.join(root, "tmp/corpus/result.json")));
  const outDir = path.dirname(outFile);
  let repos = loadCorpus().filter((r) => (lang === "all" || r.language === lang) && (!only.length || only.includes(r.name)));
  if (repos.length === 0) throw new Error("no repositories selected");
  const prep = fs.mkdtempSync(path.join(process.env.RUNNER_TEMP || os.tmpdir(), "fldlint-corpus-prep-"));
  const cleanup = () => {
    for (const c of children) kill(c);
    fs.rmSync(prep, { recursive: true, force: true });
  };
  for (const sig of ["SIGINT", "SIGTERM"]) process.on(sig, () => (cleanup(), process.exit(130)));
  const results = {};
  try {
    const langs = new Set(repos.map((r) => r.language));
    if (langs.has("js")) await prepareJs();
    if (langs.has("go")) await prepareGo(prep);
    if (langs.has("java")) await prepareJava();
    for (const repo of repos) {
      process.stderr.write(`corpus: ${repo.language}/${repo.name} @ ${repo.sha.slice(0, 12)} ...\n`);
      const r = await runRepo(repo, outDir);
      results[repo.name] = r;
      process.stderr.write(
        `corpus:   ${r.status} in ${(r.durationMs / 1000).toFixed(1)}s, findings ${r.counts?.total ?? "-"}, ` +
          `build pre/post ${buildWord(r.build.pre)}/${buildWord(r.build.post)}${r.error ? `, ${r.error.split("\n")[0]}` : ""}\n`,
      );
    }
  } finally {
    cleanup();
  }
  fs.mkdirSync(outDir, { recursive: true });
  const doc = { schema: 1, generatedAt: new Date().toISOString(), repos: sortKeys(results) };
  fs.writeFileSync(outFile, JSON.stringify(doc, null, 2) + "\n");
  process.stderr.write(`corpus: wrote ${path.relative(root, outFile)}\n`);
}

const buildWord = (b) => (!b ? "-" : b.skipped ? "skip" : b.ok ? "ok" : "FAIL");

// ------------------------------------------------------------------ compare

const sum = (o) => Object.values(o ?? {}).reduce((a, b) => a + b, 0);

/** @returns {{failures: string[], warnings: string[], notes: string[]}} */
export function compare(result, snapshot) {
  const failures = [];
  const warnings = [];
  const notes = [];
  for (const [name, r] of Object.entries(result.repos)) {
    const s = snapshot?.repos?.[name];
    if (r.status !== "ok") {
      warnings.push(`${name}: ${r.status}: ${r.error}`);
      continue;
    }
    const pre = r.build.pre;
    const post = r.build.post;
    if (post && post.ok === false) {
      if (pre && pre.ok === false) warnings.push(`${name}: the build already fails before fixing (${pre.cmd}), so no safety verdict`);
      else failures.push(`${name}: SAFETY BUG: ${post.cmd} fails after the fixes were applied (it passed before)`);
    }
    if (!s || s.status !== "ok") {
      if (!s) notes.push(`${name}: not in the snapshot; run \`task corpus:update\` after review`);
      if (!s && sum(r.counts.rewrites) > 0) {
        failures.push(`${name}: new repo with ${sum(r.counts.rewrites)} rewrite(s) and no snapshot entry`);
      }
      continue;
    }
    if (r.sha !== s.sha) notes.push(`${name}: pinned SHA changed (${s.sha.slice(0, 12)} -> ${r.sha.slice(0, 12)})`);
    const rules = new Set([...Object.keys(r.counts.rewrites), ...Object.keys(s.counts.rewrites)]);
    for (const rule of [...rules].sort()) {
      const now = r.counts.rewrites[rule] ?? 0;
      const was = s.counts.rewrites[rule] ?? 0;
      if (now > was) failures.push(`${name}: rewrite count for ${rule} rose ${was} -> ${now} (add a fixture, then \`task corpus:update\`)`);
      else if (now < was) notes.push(`${name}: rewrite count for ${rule} fell ${was} -> ${now}`);
    }
    for (const kind of ["skips", "noFix"]) {
      for (const k of new Set([...Object.keys(r.counts[kind]), ...Object.keys(s.counts[kind])])) {
        const now = r.counts[kind][k] ?? 0;
        const was = s.counts[kind][k] ?? 0;
        if (now !== was) notes.push(`${name}: ${kind} ${k} ${was} -> ${now}`);
      }
    }
  }
  return { failures, warnings, notes };
}

function readJson(p) {
  return fs.existsSync(p) ? JSON.parse(fs.readFileSync(p, "utf8")) : null;
}

const stripBuild = (b) => (b ? Object.fromEntries(Object.entries(b).filter(([k]) => k !== "log")) : b);

/** Result -> committed snapshot: drop volatile fields (times, logs). Refuses a partial or failed run. */
function cmdSnapshot(argv) {
  const result = readJson(path.resolve(argv[0] ?? "tmp/corpus/result.json"));
  const want = loadCorpus().map((r) => r.name);
  const bad = want.filter((n) => result.repos[n]?.status !== "ok");
  if (bad.length) throw new Error(`cannot snapshot: not every repo ran ok (${bad.join(", ")})`);
  const repos = {};
  for (const n of want) {
    const { durationMs, error, deps, ...r } = result.repos[n];
    repos[n] = { ...r, build: { pre: stripBuild(r.build.pre), post: stripBuild(r.build.post) } };
  }
  const out = path.resolve(argv[1] ?? path.join(root, "spec/corpus-snapshot.json"));
  fs.writeFileSync(out, JSON.stringify(sortKeys({ schema: 1, repos }), null, 2) + "\n");
  console.log(`corpus: wrote ${path.relative(root, out)}`);
}

function cmdCompare(argv) {
  const result = readJson(path.resolve(argv[0] ?? "tmp/corpus/result.json"));
  const snapshot = readJson(path.resolve(argv[1] ?? path.join(root, "spec/corpus-snapshot.json")));
  const { failures, warnings, notes } = compare(result, snapshot);
  for (const w of warnings) console.log(`warning: ${w}`);
  for (const n of notes) console.log(`note: ${n}`);
  for (const f of failures) console.log(`FAIL: ${f}`);
  console.log(failures.length ? `corpus: FAILED (${failures.length})` : "corpus: ok");
  process.exitCode = failures.length ? 1 : 0;
}

function cmdSummary(argv) {
  const result = readJson(path.resolve(argv[0] ?? "tmp/corpus/result.json"));
  const snapshot = readJson(path.resolve(argv[1] ?? path.join(root, "spec/corpus-snapshot.json")));
  const { failures, warnings, notes } = compare(result, snapshot);
  const fmt = (o) => Object.entries(o ?? {}).map(([k, v]) => `${k}: ${v}`).join(", ") || "-";
  const lines = ["## fldlint corpus", "", `Verdict: **${failures.length ? "FAILED" : "ok"}**`, ""];
  lines.push("| Repo | Lang | Status | Rewrites | Skips | No-fix | Build pre/post | Time |", "|---|---|---|---|---|---|---|---|");
  for (const [name, r] of Object.entries(result.repos)) {
    lines.push(
      `| ${name} | ${r.language} | ${r.status} | ${fmt(r.counts?.rewrites)} | ${fmt(r.counts?.skips)} | ${fmt(r.counts?.noFix)} | ` +
        `${buildWord(r.build.pre)}/${buildWord(r.build.post)} | ${(r.durationMs / 1000).toFixed(0)}s |`,
    );
  }
  const section = (title, items) => items.length && lines.push("", `### ${title}`, "", ...items.map((i) => `- ${i}`));
  section("Failures", failures);
  section("Warnings", warnings);
  section("Notes", notes);
  console.log(lines.join("\n"));
}

const cmd = process.argv[2];
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const rest = process.argv.slice(3);
  try {
    if (cmd === "run") await cmdRun(rest);
    else if (cmd === "compare") cmdCompare(rest);
    else if (cmd === "snapshot") cmdSnapshot(rest);
    else if (cmd === "summary") cmdSummary(rest);
    else {
      console.error("usage: run.mjs run|compare|summary ...");
      process.exitCode = 2;
    }
  } catch (e) {
    console.error(`corpus: ${e.message}`);
    process.exitCode = 2;
  }
}
