import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { EmitError, emitSpecJson } from "../emit.js";
import { generateRulesTs, SPEC_JSON } from "../gen.js";
import { loadRules } from "../load.js";
import { resolveSpecRoot } from "../specRoot.js";

const root = resolveSpecRoot();
let specJson: string;
try {
  // Validates everything first: an invalid spec writes no generated file.
  specJson = emitSpecJson(root);
} catch (e) {
  if (!(e instanceof EmitError)) throw e;
  console.error(`gen: ${e.message}`);
  process.exit(1);
}
const out = resolve(dirname(fileURLToPath(import.meta.url)), "../../src/rules.gen.ts");
writeFileSync(out, generateRulesTs(loadRules(root)));
console.log(`gen: wrote ${out} from ${root}/rules.yaml`);
const jsonOut = join(root, SPEC_JSON);
mkdirSync(dirname(jsonOut), { recursive: true });
writeFileSync(jsonOut, specJson);
console.log(`gen: wrote ${jsonOut}`);

// The Go stub spec (used only when the repo spec is absent) is generated too,
// from a small YAML stub validated against the real schemas.
const stubRoot = join(root, "../go/conformance/testdata");
if (existsSync(join(stubRoot, "rules.yaml"))) {
  const stubOut = join(root, "../go/internal/spec/testdata/spec.json");
  try {
    mkdirSync(dirname(stubOut), { recursive: true });
    writeFileSync(stubOut, emitSpecJson(stubRoot, root));
  } catch (e) {
    if (!(e instanceof EmitError)) throw e;
    console.error(`gen: stub spec: ${e.message}`);
    process.exit(1);
  }
  console.log(`gen: wrote ${stubOut}`);
}
