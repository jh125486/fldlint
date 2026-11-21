import { resolveSpecRoot } from "../specRoot.js";
import { checkTags } from "../tags.js";

const args = process.argv.slice(2);
const rootArg = args.find((a) => a.startsWith("--spec-root="))?.slice("--spec-root=".length);
const root = resolveSpecRoot(rootArg);
const res = checkTags(root, { allowEmpty: args.includes("--allow-empty") });
if (res.note) {
  console.log(`spec:tags: ${res.note}`);
  process.exit(0);
}
for (const e of res.errors) console.error(`spec:tags: ${e}`);
if (res.errors.length > 0) {
  console.error(`spec:tags: FAILED with ${res.errors.length} problem(s)`);
  process.exit(1);
}
console.log(`spec:tags: OK (root=${root})`);
