import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { EmitError, emitSpecJson } from "../emit.js";
import { generateRulesTs, SPEC_JSON } from "../gen.js";
import { loadRules } from "../load.js";
import { resolveSpecRoot } from "../specRoot.js";

const root = resolveSpecRoot();

// Validate and render everything in memory first, then write: a failure
// anywhere leaves no partially updated output.
const outputs: { path: string; content: string }[] = [];
try {
  outputs.push({ path: join(root, SPEC_JSON), content: emitSpecJson(root) });
  const rulesTs = resolve(dirname(fileURLToPath(import.meta.url)), "../../src/rules.gen.ts");
  outputs.push({ path: rulesTs, content: generateRulesTs(loadRules(root)) });
  // The Go stub spec (used only when the repo spec is absent) is generated too,
  // from a small YAML stub validated against the real schemas.
  const stubRoot = join(root, "../go/conformance/testdata");
  if (existsSync(join(stubRoot, "rules.yaml"))) {
    outputs.push({ path: join(root, "../go/internal/spec/testdata/spec.json"), content: emitSpecJson(stubRoot, root) });
  }
} catch (e) {
  if (!(e instanceof EmitError)) throw e;
  console.error(`gen: ${e.message}`);
  process.exit(1);
}
for (const o of outputs) {
  mkdirSync(dirname(o.path), { recursive: true });
  writeFileSync(o.path, o.content);
  console.log(`gen: wrote ${o.path}`);
}
