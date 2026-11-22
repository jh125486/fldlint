import { readFileSync, writeFileSync } from "node:fs";

/** Malformed or unsupported baseline; the CLI maps this to exit code 2. */
export class BaselineError extends Error {
  override name = "BaselineError";
}

const FINGERPRINT = /^[0-9a-f]{32}$/;

/** Serialize: sorted ascending, deduplicated, two-space indent, trailing newline. */
export function formatBaseline(fingerprints: Iterable<string>): string {
  const list = [...new Set(fingerprints)];
  for (const fp of list) if (!FINGERPRINT.test(fp)) throw new BaselineError(`invalid fingerprint: ${JSON.stringify(fp)}`);
  list.sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
  return JSON.stringify({ version: 1, fingerprints: list }, null, 2) + "\n";
}

/** Strictly parse baseline JSON text; throws {@link BaselineError}. */
export function parseBaseline(text: string): Set<string> {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch (e) {
    throw new BaselineError(`not valid JSON: ${(e as Error).message}`);
  }
  if (typeof data !== "object" || data === null || Array.isArray(data)) throw new BaselineError("expected a JSON object");
  const obj = data as Record<string, unknown>;
  for (const k of Object.keys(obj)) {
    if (k !== "version" && k !== "fingerprints") throw new BaselineError(`unknown field ${JSON.stringify(k)}`);
  }
  if (!Object.hasOwn(obj, "version") || !Object.hasOwn(obj, "fingerprints")) {
    throw new BaselineError(`both "version" and "fingerprints" are required`);
  }
  if (typeof obj.version !== "number") {
    throw new BaselineError(`"version" must be the number 1 (flaglint baseline files are not accepted)`);
  }
  if (obj.version !== 1) throw new BaselineError(`unsupported baseline version ${obj.version}`);
  const fps = obj.fingerprints;
  if (!Array.isArray(fps)) throw new BaselineError(`"fingerprints" must be an array`);
  for (const fp of fps) {
    if (typeof fp !== "string" || !FINGERPRINT.test(fp)) {
      throw new BaselineError(`invalid fingerprint ${JSON.stringify(fp)}: expected 32 lowercase hex characters`);
    }
  }
  return new Set(fps as string[]);
}

/** Read and validate a baseline file. A missing or unreadable file is a {@link BaselineError} too. */
export function readBaseline(path: string): Set<string> {
  let text: string;
  try {
    text = readFileSync(path, "utf8");
  } catch (e) {
    throw new BaselineError(`cannot read baseline ${path}: ${(e as Error).message}`);
  }
  try {
    return parseBaseline(text);
  } catch (e) {
    if (e instanceof BaselineError) throw new BaselineError(`${path}: ${e.message}`);
    throw e;
  }
}

export function writeBaseline(path: string, fingerprints: Iterable<string>): void {
  const text = formatBaseline(fingerprints);
  try {
    writeFileSync(path, text);
  } catch (e) {
    throw new BaselineError(`cannot write baseline ${path}: ${(e as Error).message}`);
  }
}
