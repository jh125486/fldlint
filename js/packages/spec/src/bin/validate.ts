import { parseArgs, resolveSpecRoot } from "../specRoot.js";
import { validateSpec } from "../validate.js";

let root: string;
let allowMissing: boolean;
try {
  const args = parseArgs(process.argv.slice(2), ["--allow-missing-schema"]);
  root = resolveSpecRoot(args.specRoot);
  allowMissing = args.flags.has("--allow-missing-schema");
} catch (e) {
  console.error(`spec:validate: error: ${(e as Error).message}`);
  process.exit(2);
}
const res = validateSpec(root);

for (const e of res.errors) console.error(`spec:validate: ${e}`);
if (res.missingSchemas.length > 0) {
  const msg = `schema file(s) not found under ${root}: ${res.missingSchemas.join(", ")}`;
  if (allowMissing) console.error(`spec:validate: warning: ${msg} (allowed by --allow-missing-schema)`);
  else {
    console.error(`spec:validate: error: ${msg}`);
    process.exit(1);
  }
}
if (res.errors.length > 0) {
  console.error(`spec:validate: FAILED with ${res.errors.length} error(s)`);
  process.exit(1);
}
console.log(`spec:validate: OK (root=${root}, cases checked=${res.casesChecked})`);
