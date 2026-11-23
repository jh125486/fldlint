import { runCodemod } from "./run.js";

/** The slice of Codemod's `SgRoot` this transform uses. */
interface SgRootLike {
  filename(): string;
  source?(): string;
  root(): { text(): string };
}

/** Path as the output-format contract wants it: relative to the target dir, `/` separators. */
function displayPath(name: string, targetDir?: string): string {
  if (name === "anonymous") return "input.ts";
  if (targetDir && name.startsWith(targetDir)) return name.slice(targetDir.length).replace(/^[\\/]+/, "");
  return name;
}

/**
 * JSSG entry point (`npx codemod`). Returns the new source, or `null` to leave
 * the file untouched. Skips are logged one per line to stdout.
 */
export default function transform(sg: SgRootLike, options?: { targetDir?: string }): string | null {
  const source = sg.source ? sg.source() : sg.root().text();
  const { output, reports } = runCodemod(source, displayPath(sg.filename(), options?.targetDir));
  for (const line of reports) console.log(line);
  return output;
}
