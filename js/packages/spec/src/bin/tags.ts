import { parseArgs, resolveSpecRoot } from "../specRoot.js";
import { checkTags } from "../tags.js";

let root: string;
let allowEmpty: boolean;
try {
  const args = parseArgs(process.argv.slice(2), ["--allow-empty"]);
  root = resolveSpecRoot(args.specRoot);
  allowEmpty = args.flags.has("--allow-empty");
} catch (e) {
  console.error(`spec:tags: error: ${(e as Error).message}`);
  process.exit(2);
}
const res = checkTags(root, { allowEmpty });
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
