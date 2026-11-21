import { resolveSpecRoot } from "../specRoot.js";
import { validateSpec } from "../validate.js";

const args = process.argv.slice(2);
const allowMissing = args.includes("--allow-missing-schema");
const rootArg = args.find((a) => a.startsWith("--spec-root="))?.slice("--spec-root=".length);
const root = resolveSpecRoot(rootArg);
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
