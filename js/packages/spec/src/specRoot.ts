import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

/** Absolute path of the bundled stub spec (used only when no spec root is given at all). */
export const STUB_SPEC_ROOT = resolve(here, "../testdata");

const REQUIRED_FILES = ["rules.yaml", "schema/rules.schema.json", "schema/fixture.schema.json"];

/** Throws when an explicitly given spec root is missing or lacks rules.yaml / the schemas. */
export function assertSpecRoot(root: string, origin: string): void {
  const missing = REQUIRED_FILES.filter((f) => !existsSync(join(root, f)));
  if (missing.length > 0) {
    throw new SpecRootError(`${origin} spec root ${root} is missing or invalid (not found: ${missing.join(", ")})`);
  }
}

export class SpecRootError extends Error {}

/**
 * Resolve the spec root. Order: explicit argument, FLDLINT_SPEC_ROOT env,
 * repo-root /spec when present, else the bundled stub. An explicit argument
 * or env value that is missing or invalid is a hard error (SpecRootError);
 * the stub fallback applies only when neither is given.
 */
export function resolveSpecRoot(explicit?: string, env: Record<string, string | undefined> = process.env): string {
  if (explicit !== undefined) {
    if (explicit === "") throw new SpecRootError("--spec-root requires a non-empty path");
    const root = resolve(explicit);
    assertSpecRoot(root, "explicit");
    return root;
  }
  const fromEnv = env["FLDLINT_SPEC_ROOT"];
  if (fromEnv) {
    const root = resolve(fromEnv);
    assertSpecRoot(root, "FLDLINT_SPEC_ROOT");
    return root;
  }
  const repoSpec = resolve(here, "../../../../spec");
  if (existsSync(resolve(repoSpec, "rules.yaml"))) return repoSpec;
  return STUB_SPEC_ROOT;
}

/**
 * Parse `--spec-root=PATH` or `--spec-root PATH` plus boolean flags. Unknown
 * arguments are a usage error, so a typo can never silently select the fallback.
 */
export function parseArgs(argv: string[], flags: string[]): { specRoot?: string; flags: Set<string> } {
  const out: { specRoot?: string; flags: Set<string> } = { flags: new Set() };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]!;
    if (a.startsWith("--spec-root=")) out.specRoot = a.slice("--spec-root=".length);
    else if (a === "--spec-root") {
      const v = argv[++i];
      if (v === undefined) throw new SpecRootError("--spec-root requires a value");
      out.specRoot = v;
    } else if (flags.includes(a)) out.flags.add(a);
    else throw new SpecRootError(`unknown argument ${a}`);
  }
  return out;
}
