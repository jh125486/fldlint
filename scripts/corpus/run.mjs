#!/usr/bin/env node
// Real-repo corpus runner (issue #21, Part 2). Read scripts/corpus/README.md.
//
//   node scripts/corpus/run.mjs run     [--lang js|go|java|all] [--only a,b] [--out file]
//   node scripts/corpus/run.mjs compare <result.json> [snapshot.json]
//   node scripts/corpus/run.mjs selftest [--lang go|js|java]   (task corpus:selftest)
//   node scripts/corpus/run.mjs summary <result.json> [snapshot.json]
//
// Uses only Node built-ins (the corpus is JSON; js/ is needed only for the built ESLint plugin).
// Every repo is cloned into a temp dir, analysed in report mode, fixed in that
// temp copy only, built, and deleted. Nothing is ever pushed or commented.
import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { compare, sum } from "./compare.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const jsDir = path.join(root, "js");
const requireJs = createRequire(path.join(jsDir, "package.json"));
const MAVEN_PLUGIN = "org.openrewrite.maven:rewrite-maven-plugin:6.46.1";
// Find only: run together with Migrate (as java/smoke does) it sees the already rewritten tree and
// exports no rows for the rewritable calls, so the report pass is a separate, Find-only dryRun.
const RECIPES_ALL = "io.github.jh125486.fldlint.FindLaunchDarklyUsage";
const RECIPE_FIX = "io.github.jh125486.fldlint.MigrateLaunchDarklyToOpenFeature";
const LANGS = ["js", "go", "java"];

// ---------------------------------------------------------------- processes

const children = new Set();
const tmpDirs = new Set();

/** A temp dir that the signal handler also removes. */
function makeTmp(prefix) {
  const d = fs.mkdtempSync(path.join(process.env.RUNNER_TEMP || os.tmpdir(), prefix));
  tmpDirs.add(d);
  return d;
}
function removeTmp(d) {
  fs.rmSync(d, { recursive: true, force: true });
  tmpDirs.delete(d);
}
function killAll() {
  for (const c of children) kill(c);
  for (const d of tmpDirs) fs.rmSync(d, { recursive: true, force: true });
  tmpDirs.clear();
}

// Child environments are built from an allowlist: nothing like GH_TOKEN, AWS_*, SSH_AUTH_SOCK or
// npm tokens reaches a process, whatever the parent has. Per-run caches (set by initCaches) keep
// third-party builds from touching the caches a workflow may persist.
const ENV_ALLOW = [
  "PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "TERM", "TMPDIR", "TMP", "TEMP", "RUNNER_TEMP",
  "JAVA_HOME", "GOROOT", "GOPATH", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
  "SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS", "MISE_DATA_DIR", "MISE_CONFIG_DIR", "MISE_CACHE_DIR",
];
let caches = null;
function initCaches(dir) {
  caches = {
    gomod: path.join(dir, "gomod"),
    gocache: path.join(dir, "gocache"),
    m2: path.join(dir, "m2"),
    npm: path.join(dir, "npm"),
    // npm refuses the same file for the user and global config, so each gets its own empty one.
    npmUser: path.join(dir, "npmrc-user"),
    npmGlobal: path.join(dir, "npmrc-global"),
  };
  fs.writeFileSync(caches.npmUser, "");
  fs.writeFileSync(caches.npmGlobal, "");
}

/** @param {boolean} trusted true for builds of this checkout, false (default) for third-party code */
export function childEnv(trusted, extra = {}, base = process.env) {
  const env = {};
  for (const k of ENV_ALLOW) if (base[k] !== undefined) env[k] = base[k];
  env.CI = "1";
  env.GIT_TERMINAL_PROMPT = "0";
  env.GIT_CONFIG_NOSYSTEM = "1";
  env.GIT_CONFIG_GLOBAL = "/dev/null";
  env.GOTOOLCHAIN = "local";
  if (!trusted) {
    // Environment beats a repo's .npmrc, go.env-style files and user config, so a cloned repo
    // cannot redirect installs to another registry or turn lifecycle scripts back on.
    Object.assign(env, {
      NPM_CONFIG_REGISTRY: "https://registry.npmjs.org/",
      NPM_CONFIG_IGNORE_SCRIPTS: "true",
      NPM_CONFIG_GIT: "/nonexistent/git",
      NPM_CONFIG_AUDIT: "false",
      NPM_CONFIG_FUND: "false",
      GOENV: "off",
      GOFLAGS: "-modcacherw",
    });
    if (caches) {
      Object.assign(env, {
        GOMODCACHE: caches.gomod,
        GOCACHE: caches.gocache,
        NPM_CONFIG_CACHE: caches.npm,
        NPM_CONFIG_USERCONFIG: caches.npmUser,
        NPM_CONFIG_GLOBALCONFIG: caches.npmGlobal,
      });
    }
  }
  return { ...env, ...extra };
}

/** Run a command; kills its whole process group at the deadline. Never throws. */
function sh(cmd, args, { cwd, env, deadline, input, trusted = false } = {}) {
  return new Promise((resolve) => {
    const budget = deadline ? deadline - Date.now() : Infinity;
    if (budget <= 0) return resolve({ code: null, stdout: "", stderr: "", timedOut: true });
    const child = spawn(cmd, args, {
      cwd,
      env: childEnv(trusted, env),
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
/** kind "infra": clone, fetch, dependency or environment trouble (a warning). kind "tool": fldlint itself misbehaved (a failure). */
class StepError extends Error {
  constructor(message, kind = "infra") {
    super(message);
    this.kind = kind;
  }
}
const must = (r, what, kind = "infra") => {
  if (r.timedOut) throw new Timeout(what);
  if (r.code !== 0) throw new StepError(`${what} failed (exit ${r.code}): ${tail(r.stderr || r.stdout, 600)}`, kind);
  return r;
};

/** git with hooks disabled (the clone is untrusted); the environment already ignores user and system config. */
const git = (args, opts = {}) => sh("git", ["-c", "core.hooksPath=/dev/null", ...args], opts);

// ------------------------------------------------------------------- corpus

/** Everything from corpus.json is validated here; it is the only source of names, URLs and SHAs. */
function loadCorpus() {
  const doc = JSON.parse(fs.readFileSync(path.join(root, "spec/corpus.json"), "utf8"));
  const repos = [];
  const seen = new Set();
  const int = (v, max) => Number.isInteger(v) && v > 0 && v <= max;
  for (const lang of LANGS) {
    for (const r of doc[lang] ?? []) {
      for (const k of ["name", "url", "sha", "language", "license", "note"]) {
        if (typeof r[k] !== "string" || r[k] === "") throw new Error(`corpus.json: ${lang}/${r.name}: ${k} must be a non-empty string`);
      }
      if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(r.name)) throw new Error(`corpus.json: bad name ${JSON.stringify(r.name)}`);
      if (r.language !== lang) throw new Error(`corpus.json: ${r.name}: language ${r.language} under ${lang}`);
      if (!/^[0-9a-f]{40}$/.test(r.sha)) throw new Error(`corpus.json: ${r.name}: sha must be 40 hex chars`);
      if (!/^https:\/\/github\.com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(r.url) || r.url.endsWith("/.") || r.url.includes("/.."))
        throw new Error(`corpus.json: ${r.name}: url must be https://github.com/<owner>/<repo>`);
      if (!int(r.timeout, 7200)) throw new Error(`corpus.json: ${r.name}: timeout must be 1..7200 seconds`);
      if (!int(r.depth, 50)) throw new Error(`corpus.json: ${r.name}: depth must be 1..50`);
      if (seen.has(r.name)) throw new Error(`corpus.json: duplicate name ${r.name}`);
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
  must(await git(["init", "-q", dir], { deadline }), "git init");
  must(await git(["remote", "add", "origin", repo.url], { cwd: dir, deadline }), "git remote add");
  must(
    await git(["-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "fetch", "-q", "--depth", String(repo.depth), "origin", repo.sha], { cwd: dir, deadline }),
    "git fetch",
  );
  must(await git(["checkout", "-q", "FETCH_HEAD"], { cwd: dir, deadline }), "git checkout");
  const head = (await git(["rev-parse", "HEAD"], { cwd: dir, deadline })).stdout.trim();
  if (head !== repo.sha) throw new StepError(`checked out ${head}, expected ${repo.sha}`);
}

/** Commit the dependency prep so later diffs show only fldlint's own edits. Must succeed, or prep would leak into the fix. */
async function commitBaseline(dir, deadline) {
  must(await git(["add", "-A"], { cwd: dir, deadline }), "git add (baseline)");
  must(
    await git(["-c", "user.name=corpus", "-c", "user.email=corpus@invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "prep"], { cwd: dir, deadline }),
    "git commit (baseline)",
  );
}

async function changedFiles(dir, deadline) {
  const r = must(await git(["status", "--porcelain", "--untracked-files=no"], { cwd: dir, deadline }), "git status");
  return r.stdout.split("\n").filter(Boolean).length;
}
async function fixPatch(dir, deadline) {
  return must(await git(["diff", "--no-ext-diff", "--no-textconv"], { cwd: dir, deadline }), "git diff").stdout;
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
  // Trusted: this checkout's own build. Lifecycle scripts must run (the workspaces build through pre-hooks).
  const r = await sh("npm", ["run", "build", "--silent"], { cwd: jsDir, trusted: true });
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
  const r = await sh(eslint, args, { cwd: dir, deadline, trusted: true });
  if (r.timedOut) throw new Timeout("eslint");
  if (r.code !== 0 && r.code !== 1) throw new StepError(`eslint exit ${r.code}: ${tail(r.stderr || r.stdout, 600)}`, "tool");
  let files;
  try {
    files = JSON.parse(r.stdout);
  } catch {
    throw new StepError(`eslint did not print JSON: ${tail(r.stdout || r.stderr, 600)}`, "tool");
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
  const r = await sh("go", ["build", "-o", fldlintBin, "./cmd/fldlint"], { cwd: path.join(root, "go"), trusted: true });
  if (r.code !== 0) throw new Error(`building fldlint failed: ${tail(r.stderr)}`);
}

async function analyzeGo(repo, dir, tmp, deadline, out) {
  const env = { ...(repo.env ?? {}) };
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
  // If the repo does not build on its own, fldlint failing to load it is the repo's problem (infra).
  const kind = out.build.pre.ok ? "tool" : "infra";
  const r = await sh(fldlintBin, ["-json", "./..."], { cwd: dir, deadline, env });
  if (r.timedOut) throw new Timeout("fldlint -json");
  if (r.code !== 0) throw new StepError(`fldlint -json exit ${r.code}: ${tail(r.stderr || r.stdout, 600)}`, kind);
  const counts = newCounts();
  const text = r.stdout.trim();
  let doc;
  try {
    doc = text ? JSON.parse(text) : {};
  } catch {
    throw new StepError("fldlint -json did not print JSON", kind);
  }
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
  must(await sh(fldlintBin, ["-fix", "./..."], { cwd: dir, deadline, env }), "fldlint -fix", kind);
  out.fixedFiles = await changedFiles(dir, deadline);
  out.patch = await fixPatch(dir, deadline);
  out.build.post =
    out.fixedFiles === 0
      ? skippedBuild("nothing changed")
      : buildResult(await sh("go", ["build", "./..."], { cwd: dir, deadline, env }), "go build ./...");
}

// --------------------------------------------------------------------- Java

let javaCoords;
let javaVersion;
async function prepareJava() {
  const javaDir = path.join(root, "java");
  const mvnw = path.join(javaDir, "mvnw");
  // The recipes (and everything Maven fetches for them) go into the per-run local repo that the
  // third-party builds also use, so no persisted ~/.m2 is touched by third-party code.
  const repo = `-Dmaven.repo.local=${caches.m2}`;
  let r = await sh(mvnw, ["-B", "-q", "-DskipTests", repo, "install"], { cwd: javaDir, trusted: true });
  if (r.code !== 0) throw new Error(`installing the recipes failed: ${tail(r.stderr || r.stdout)}`);
  r = await sh(
    mvnw,
    ["-q", "-B", repo, "org.apache.maven.plugins:maven-help-plugin:3.6.0:evaluate", "-Dexpression=project.version", "-DforceStdout"],
    { cwd: javaDir, trusted: true },
  );
  if (r.code !== 0) throw new Error(`reading the recipe version failed: ${tail(r.stderr)}`);
  javaVersion = r.stdout.trim();
  javaCoords = `io.github.jh125486:fldlint-rewrite:${javaVersion}`;
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

/** Regular files only: a cloned repo could plant a symlink where a result file is expected. */
function findFiles(dir, name, acc = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    const st = fs.lstatSync(p);
    if (st.isSymbolicLink()) continue;
    if (st.isDirectory()) findFiles(p, name, acc);
    else if (st.isFile() && e.name.endsWith(name) && st.size < 32 * 1024 * 1024) acc.push(p);
  }
  return acc;
}

async function analyzeJava(repo, dir, tmp, deadline, out) {
  const mvnw = path.join(root, "java", "mvnw");
  if (!fs.existsSync(path.join(dir, "pom.xml"))) throw new StepError("no pom.xml at the repository root");
  const m2 = `-Dmaven.repo.local=${caches.m2}`;
  const common = ["-B", m2, "-Drewrite.recipeArtifactCoordinates=" + javaCoords, "-Dfldlint.version=" + javaVersion];
  const compile = () => sh(mvnw, ["-B", "-q", m2, "compile"], { cwd: dir, deadline });
  out.build.pre = buildResult(await compile(), "mvn -q compile");
  const kind = out.build.pre.ok ? "tool" : "infra";
  must(
    await sh(
      mvnw,
      [...common, `${MAVEN_PLUGIN}:dryRun`, `-Drewrite.activeRecipes=${RECIPES_ALL}`, "-Drewrite.exportDatatables=true"],
      { cwd: dir, deadline },
    ),
    "rewrite:dryRun",
    kind,
  );
  const counts = newCounts();
  // A successful dry run always exports the datatables directory (RecipeRunStats, even with zero
  // findings); without it the export silently broke. No LaunchDarklyUsage table means no findings.
  const tables = path.join(dir, "target", "rewrite", "datatables");
  if (!fs.existsSync(tables)) throw new StepError("dryRun succeeded but exported no datatables directory", kind);
  const csvs = findFiles(tables, "LaunchDarklyUsage.csv");
  const seen = new Set();
  for (const csv of csvs) {
    // OpenRewrite prefixes the CSV with `# @name ...` comment lines.
    const [head, ...rows] = parseCsv(fs.readFileSync(csv, "utf8")).filter((r) => !r[0].startsWith("#"));
    const col = (n) => (head ?? []).indexOf(n);
    const [iRule, iSkip, iNoFix, iFp] = [col("ruleId"), col("skip"), col("noFix"), col("fingerprint")];
    if (iRule < 0 || iSkip < 0) throw new StepError("unexpected LaunchDarklyUsage data table columns", kind);
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
    kind,
  );
  out.fixedFiles = await changedFiles(dir, deadline);
  out.patch = await fixPatch(dir, deadline);
  out.build.post = out.fixedFiles === 0 ? skippedBuild("nothing changed") : buildResult(await compile(), "mvn -q compile");
}

// ------------------------------------------------------------------- driver

const analyzers = { js: analyzeJs, go: analyzeGo, java: analyzeJava };

async function runRepo(repo, outDir) {
  const started = Date.now();
  const deadline = started + repo.timeout * 1000;
  const tmp = makeTmp(`fldlint-corpus-${repo.name}-`);
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
    // Anything that is not a classified step failure is a bug in this runner or fldlint: a tool error.
    out.errorKind = e instanceof Timeout ? "infra" : (e.kind ?? "tool");
    out.error = e instanceof Timeout ? `timed out after ${repo.timeout}s in: ${e.message}` : tail(String(e.message ?? e), 1500);
  } finally {
    if (out.patch && outDir && (out.build.post?.ok === false || process.env.CORPUS_KEEP_PATCHES === "1")) {
      fs.mkdirSync(outDir, { recursive: true });
      fs.writeFileSync(path.join(outDir, `${repo.name}.fix.patch`), out.patch);
    }
    delete out.patch;
    removeTmp(tmp);
    out.durationMs = Date.now() - started;
  }
  return out;
}

function arg(argv, name, dflt) {
  const i = argv.indexOf(name);
  return i >= 0 ? argv[i + 1] : dflt;
}

function writeResult(outFile, results) {
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  const doc = { schema: 1, generatedAt: new Date().toISOString(), repos: sortKeys(results) };
  fs.writeFileSync(outFile, JSON.stringify(doc, null, 2) + "\n");
}

async function cmdRun(argv) {
  const lang = arg(argv, "--lang", "all");
  const only = arg(argv, "--only", "")?.split(",").filter(Boolean) ?? [];
  const outFile = path.resolve(arg(argv, "--out", path.join(root, "tmp/corpus/result.json")));
  const outDir = path.dirname(outFile);
  const repos = loadCorpus().filter((r) => (lang === "all" || r.language === lang) && (!only.length || only.includes(r.name)));
  if (repos.length === 0) throw new Error("no repositories selected");
  const prep = makeTmp("fldlint-corpus-prep-");
  initCaches(prep);
  for (const sig of ["SIGINT", "SIGTERM"]) process.on(sig, () => (killAll(), process.exit(130)));
  const results = {};
  try {
    const langs = new Set(repos.map((r) => r.language));
    // A failed preparation is written as an error result for each repo, so result.json always exists.
    try {
      if (langs.has("js")) await prepareJs();
      if (langs.has("go")) await prepareGo(prep);
      if (langs.has("java")) await prepareJava();
    } catch (e) {
      for (const repo of repos) {
        results[repo.name] = { name: repo.name, language: repo.language, url: repo.url, sha: repo.sha, license: repo.license, status: "error", errorKind: "tool", error: tail(String(e.message), 1500), build: {} };
      }
      writeResult(outFile, results);
      throw e;
    }
    for (const repo of repos) {
      process.stderr.write(`corpus: ${repo.language}/${repo.name} @ ${repo.sha.slice(0, 12)} ...\n`);
      const r = await runRepo(repo, outDir);
      results[repo.name] = r;
      writeResult(outFile, results);
      process.stderr.write(
        `corpus:   ${r.status} in ${(r.durationMs / 1000).toFixed(1)}s, findings ${r.counts?.total ?? "-"}, ` +
          `build pre/post ${buildWord(r.build.pre)}/${buildWord(r.build.post)}${r.error ? `, ${r.error.split("\n")[0]}` : ""}\n`,
      );
    }
  } finally {
    killAll();
  }
  process.stderr.write(`corpus: wrote ${path.relative(root, outFile)}\n`);
}

const buildWord = (b) => (!b ? "-" : b.skipped ? "skip" : b.ok ? "ok" : "FAIL");

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
    const { durationMs, error, errorKind, deps, ...r } = result.repos[n];
    repos[n] = { ...r, build: { pre: stripBuild(r.build.pre), post: stripBuild(r.build.post) } };
  }
  const out = path.resolve(argv[1] ?? path.join(root, "spec/corpus-snapshot.json"));
  fs.writeFileSync(out, JSON.stringify(sortKeys({ schema: 1, repos }), null, 2) + "\n");
  console.log(`corpus: wrote ${path.relative(root, out)}`);
}

function cmdCompare(argv) {
  const result = readJson(path.resolve(argv[0] ?? "tmp/corpus/result.json"));
  if (!result) throw new Error("no result.json to compare");
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
  if (!result) {
    console.log("## fldlint corpus\n\nNo result.json was written.");
    return;
  }
  const snapshot = readJson(path.resolve(argv[1] ?? path.join(root, "spec/corpus-snapshot.json")));
  const { failures, warnings, notes } = compare(result, snapshot);
  const fmt = (o) => Object.entries(o ?? {}).map(([k, v]) => `${k}: ${v}`).join(", ") || "-";
  const lines = ["## fldlint corpus", "", `Verdict: **${failures.length ? "FAILED" : "ok"}**`, ""];
  lines.push("| Repo | Lang | Status | Rewrites | Skips | No-fix | Build pre/post | Time |", "|---|---|---|---|---|---|---|---|");
  for (const [name, r] of Object.entries(result.repos)) {
    lines.push(
      `| ${name} | ${r.language} | ${r.status} | ${fmt(r.counts?.rewrites)} | ${fmt(r.counts?.skips)} | ${fmt(r.counts?.noFix)} | ` +
        `${buildWord(r.build.pre)}/${buildWord(r.build.post)} | ${((r.durationMs ?? 0) / 1000).toFixed(0)}s |`,
    );
  }
  const section = (title, items) => items.length && lines.push("", `### ${title}`, "", ...items.map((i) => `- ${i}`));
  section("Failures", failures);
  section("Warnings", warnings);
  section("Notes", notes);
  console.log(lines.join("\n"));
}

// ----------------------------------------------------------------- selftest

/** Make a throwaway repo out of files copied from this checkout (never a third-party repo). */
async function localRepo(tmp, copies, deadline) {
  const top = path.join(tmp, "work");
  fs.mkdirSync(top, { recursive: true });
  for (const [from, to] of copies) {
    fs.cpSync(path.join(root, from), path.join(top, to), {
      recursive: true,
      filter: (src) => !/[\\/](target|node_modules|go\.sum)$/.test(src),
    });
  }
  must(await git(["init", "-q", top], { deadline }), "repo init");
  await commitBaseline(top, deadline);
  return top;
}

const fixture = "spec/fixtures/bool-variation/aliased-import";

const selftests = {
  // Smoke module of the golangci plugin: fixture input and golden plus SDK stubs (offline).
  async go(tmp, deadline, out) {
    const top = await localRepo(tmp, [["go/golangci/testdata/smoke", "smoke"], ["go/golangci/testdata/stubs", "stubs"]], deadline);
    await prepareGo(tmp);
    // GOPROXY=off proves the module needs no network (the stubs are replace directives).
    await analyzeGo({ env: { GOPROXY: "off" } }, path.join(top, "smoke"), tmp, deadline, out);
    const want = fs.readFileSync(path.join(root, "go/golangci/testdata/smoke/flags/flags.go.golden"), "utf8");
    const got = fs.readFileSync(path.join(top, "smoke/flags/flags.go"), "utf8");
    if (got !== want) throw new StepError("fixed flags.go differs from flags.go.golden", "tool");
  },
  // The shared aliased-import fixture; real SDK packages from npm (needs network).
  async js(tmp, deadline, out) {
    const top = await localRepo(tmp, [[`${fixture}/js/input.ts`, "src/darkmode.ts"]], deadline);
    fs.writeFileSync(
      path.join(top, "package.json"),
      JSON.stringify({ name: "selftest", private: true, type: "module", dependencies: { "@launchdarkly/node-server-sdk": "^9.0.0", "@openfeature/server-sdk": "^1.0.0" } }),
    );
    fs.writeFileSync(
      path.join(top, "tsconfig.json"),
      JSON.stringify({ compilerOptions: { target: "ES2022", module: "NodeNext", moduleResolution: "NodeNext", strict: true, skipLibCheck: true, noEmit: true }, include: ["src"] }),
    );
    // A hostile .npmrc: if it were honoured, npm install would fail against this dead registry.
    fs.writeFileSync(path.join(top, ".npmrc"), "registry=http://127.0.0.1:9/\n");
    // A truly clean build: dist/ of every workspace is removed first, so a regression in the build
    // (for example lifecycle scripts being disabled) cannot hide behind stale output.
    for (const pkg of fs.readdirSync(path.join(jsDir, "packages"))) {
      fs.rmSync(path.join(jsDir, "packages", pkg, "dist"), { recursive: true, force: true });
      fs.rmSync(path.join(jsDir, "packages", pkg, "tsconfig.build.tsbuildinfo"), { force: true });
    }
    await prepareJs();
    for (const pkg of ["eslint-plugin", "core", "spec"]) {
      if (!fs.existsSync(path.join(jsDir, "packages", pkg, "dist"))) throw new StepError(`clean build did not produce packages/${pkg}/dist`, "tool");
    }
    await analyzeJs({}, top, tmp, deadline, out);
    const want = fs.readFileSync(path.join(root, fixture, "js/output.ts"), "utf8");
    if (fs.readFileSync(path.join(top, "src/darkmode.ts"), "utf8") !== want) throw new StepError("fixed darkmode.ts differs from output.ts", "tool");
  },
  // java/smoke: real LaunchDarkly and OpenFeature jars from Maven (needs network or a warm local repo).
  async java(tmp, deadline, out) {
    const top = await localRepo(tmp, [["java/smoke", "smoke"]], deadline);
    await prepareJava();
    await analyzeJava({}, path.join(top, "smoke"), tmp, deadline, out);
  },
};

async function cmdSelftest(argv) {
  const only = arg(argv, "--lang", "all");
  const langs = LANGS.filter((l) => only === "all" || only === l);
  if (langs.length === 0) throw new Error(`unknown --lang ${only}`);
  const timeout = 900_000;
  for (const sig of ["SIGINT", "SIGTERM"]) process.on(sig, () => (killAll(), process.exit(130)));
  let failed = 0;
  for (const lang of langs) {
    const tmp = makeTmp(`fldlint-corpus-self-${lang}-`);
    initCaches(tmp);
    const out = { build: {} };
    const t0 = Date.now();
    let problem;
    try {
      await selftests[lang](tmp, Date.now() + timeout, out);
      const rewrites = sum(out.counts?.rewrites);
      if (!(rewrites > 0)) problem = `expected rewrites > 0, got ${rewrites}: ${JSON.stringify(out.counts)}`;
      else if (!(out.fixedFiles > 0)) problem = "the fix changed no files";
      else if (out.build.pre?.ok !== true) problem = `build before the fix: ${JSON.stringify(out.build.pre)}`;
      else if (out.build.post?.ok !== true) problem = `post-fix build: ${JSON.stringify(out.build.post)}`;
    } catch (e) {
      problem = String(e.message ?? e);
    } finally {
      removeTmp(tmp);
    }
    const secs = ((Date.now() - t0) / 1000).toFixed(1);
    if (problem) (failed++, console.log(`selftest ${lang}: FAIL (${secs}s): ${problem}`));
    else console.log(`selftest ${lang}: ok (${secs}s): rewrites ${JSON.stringify(out.counts.rewrites)}, ${out.fixedFiles} file(s) fixed, post-fix build ok`);
  }
  process.exitCode = failed ? 1 : 0;
}

const cmd = process.argv[2];
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const rest = process.argv.slice(3);
  try {
    if (cmd === "run") await cmdRun(rest);
    else if (cmd === "compare") cmdCompare(rest);
    else if (cmd === "snapshot") cmdSnapshot(rest);
    else if (cmd === "selftest") await cmdSelftest(rest);
    else if (cmd === "summary") cmdSummary(rest);
    else {
      console.error("usage: run.mjs run|compare|snapshot|selftest|summary ...");
      process.exitCode = 2;
    }
  } catch (e) {
    console.error(`corpus: ${e.message}`);
    process.exitCode = 2;
  }
}
