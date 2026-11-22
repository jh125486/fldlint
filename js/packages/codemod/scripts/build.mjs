// Bundles src/transform.ts (with @fldlint/core, @typescript-eslint/parser and
// TypeScript) into ONE file for Codemod's QuickJS + LLRT runtime, which cannot
// resolve node_modules. The output is deterministic: no timestamps, no absolute
// paths. `--check` rebuilds into memory and fails if it differs from dist/.
import { createHash } from "node:crypto";
import { builtinModules } from "node:module";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const outfile = join(root, "dist", "transform.js");
const builtins = new Set(builtinModules);

// LLRT lacks or restricts a few things the parser's dependency tree touches at
// load time. Each stub is for code that is never executed on our path.
const STUBS = {
  // @typescript-eslint/utils re-exports eslint classes it never uses here.
  eslint: "class A {}; module.exports = new Proxy({}, { get: (_t, k) => (k === '__esModule' ? false : A) });",
  jiti: "module.exports = {};",
  // `debug` calls tty.isatty at load; logging is never enabled.
  debug:
    "const f = () => { const g = () => {}; g.enabled = false; g.extend = () => f(); return g; };" +
    "f.enable = () => {}; f.disable = () => ''; f.enabled = () => false; module.exports = f;",
  // Not present in LLRT; typescript only touches it for profiling.
  inspector: "module.exports = {};",
};

const shims = {
  name: "codemod-runtime-shims",
  setup(b) {
    b.onResolve({ filter: /^(eslint|jiti)(\/.*)?$/ }, (a) => ({ path: a.path.split("/")[0], namespace: "stub" }));
    b.onResolve({ filter: /^debug$/ }, () => ({ path: "debug", namespace: "stub" }));
    b.onResolve({ filter: /^(node:)?inspector$/ }, () => ({ path: "inspector", namespace: "stub" }));
    b.onLoad({ filter: /.*/, namespace: "stub" }, (a) => ({ contents: STUBS[a.path], loader: "js" }));
    // Bare CJS require("fs") fails in LLRT; route builtins through node: ESM.
    b.onResolve({ filter: /.*/ }, (a) => {
      if (a.kind !== "require-call") return;
      const n = a.path.replace(/^node:/, "");
      if (builtins.has(n)) return { path: n, namespace: "builtin" };
    });
    b.onLoad({ filter: /.*/, namespace: "builtin" }, (a) => ({
      contents: `import * as m from "node:${a.path}"; module.exports = m;`,
      loader: "js",
    }));
  },
};

async function bundle() {
  const result = await build({
    entryPoints: [join(root, "src", "transform.ts")],
    absWorkingDir: root,
    bundle: true,
    write: false,
    format: "esm",
    platform: "node",
    target: "es2022",
    legalComments: "none",
    logLevel: "warning",
    plugins: [shims],
  });
  return result.outputFiles[0].text;
}

const text = await bundle();
const sha = createHash("sha256").update(text).digest("hex");

if (text.includes(root)) {
  console.error("build: bundle contains an absolute path; it would not be reproducible");
  process.exit(1);
}

if (process.argv.includes("--check")) {
  // Never writes dist/. Two independent builds must be byte-identical
  // (determinism), and an existing dist/transform.js must equal them (it is not
  // stale relative to the sources, lockfile or this script).
  const again = await bundle();
  if (again !== text) {
    console.error("build:check: two builds of the same inputs differ (non-deterministic)");
    process.exit(1);
  }
  let onDisk = null;
  try {
    onDisk = readFileSync(outfile, "utf8");
  } catch {
    // no dist yet: nothing to compare against
  }
  if (onDisk !== null && onDisk !== text) {
    console.error("build:check: dist/transform.js is stale: it differs from a fresh build");
    process.exit(1);
  }
  console.log(`build:check: ok sha256=${sha}${onDisk === null ? " (no dist/ to compare)" : ""}`);
} else {
  mkdirSync(dirname(outfile), { recursive: true });
  writeFileSync(outfile, text);
  console.log(`build: ${outfile} (${text.length} bytes) sha256=${sha}`);
}
