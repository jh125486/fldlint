import { mkdirSync, writeFileSync } from "node:fs";
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
