import { existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

/** Absolute path of the bundled stub spec (used when the real spec dir is missing). */
export const STUB_SPEC_ROOT = resolve(here, "../testdata");

/**
 * Resolve the spec root. Order: explicit argument, FLDLINT_SPEC_ROOT env,
 * repo-root /spec when present, else the bundled stub.
 */
export function resolveSpecRoot(explicit?: string, env: Record<string, string | undefined> = process.env): string {
  if (explicit) return resolve(explicit);
  const fromEnv = env["FLDLINT_SPEC_ROOT"];
  if (fromEnv) return resolve(fromEnv);
  const repoSpec = resolve(here, "../../../../spec");
  if (existsSync(resolve(repoSpec, "rules.yaml"))) return repoSpec;
  return STUB_SPEC_ROOT;
}
