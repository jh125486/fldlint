import { writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { generateRulesTs } from "../gen.js";
import { loadRules } from "../load.js";
import { resolveSpecRoot } from "../specRoot.js";

const root = resolveSpecRoot();
const out = resolve(dirname(fileURLToPath(import.meta.url)), "../../src/rules.gen.ts");
writeFileSync(out, generateRulesTs(loadRules(root)));
console.log(`gen: wrote ${out} from ${root}/rules.yaml`);
