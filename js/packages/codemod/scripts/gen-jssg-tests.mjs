// Generates `npx codemod jssg test` case directories from the shared fixtures:
// spec/fixtures/**/js/{input.ts,output.ts}. A fixture without output.ts must
// come out unchanged, so expected == input. The output dirs are gitignored.
import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import { discoverCases, resolveSpecRoot } from "@fldlint/spec";
import { rewrite } from "@fldlint/core";

const pkg = new URL("..", import.meta.url).pathname;
const ts = join(pkg, ".jssg-tests");
const tsx = join(pkg, ".jssg-tests-tsx");
rmSync(ts, { recursive: true, force: true });
rmSync(tsx, { recursive: true, force: true });

let n = 0;
for (const c of discoverCases(resolveSpecRoot())) {
  const dir = join(c.dir, "js");
  if (!existsSync(join(dir, "input.ts"))) continue;
  const out = join(ts, relative(join(resolveSpecRoot(), "fixtures"), c.dir).replaceAll("/", "__"));
  mkdirSync(out, { recursive: true });
  cpSync(join(dir, "input.ts"), join(out, "input.ts"));
  cpSync(existsSync(join(dir, "output.ts")) ? join(dir, "output.ts") : join(dir, "input.ts"), join(out, "expected.ts"));
  n++;
}

// JSX needs the tsx parser and a .tsx file name: one hand-made case.
const jsxIn = readFileSync(join(resolveSpecRoot(), "fixtures/bool-variation/static-key-double-quote/js/input.ts"), "utf8") +
  "\nexport const el = <div>{String(1)}</div>;\n";
const out = join(tsx, "jsx-element");
mkdirSync(out, { recursive: true });
writeFileSync(join(out, "input.tsx"), jsxIn);
writeFileSync(join(out, "expected.tsx"), rewrite(jsxIn, "input.tsx"));
console.log(`gen-jssg-tests: ${n} fixture cases + 1 tsx case`);
