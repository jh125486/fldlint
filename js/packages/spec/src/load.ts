import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { parse } from "yaml";
import type { RulesFile } from "./types.js";

export function loadYaml(path: string): unknown {
  return parse(readFileSync(path, "utf8"));
}

export function loadRules(specRoot: string): RulesFile {
  return loadYaml(join(specRoot, "rules.yaml")) as RulesFile;
}

export interface CaseFile {
  /** Path relative to the spec root, with `/` separators. */
  relPath: string;
  /** Absolute path of the case directory. */
  dir: string;
  /** Parsed YAML (unvalidated), or undefined when parsing failed. */
  data: unknown;
  /** YAML parse error message, if any. */
  parseError?: string;
}

/** Discover spec/fixtures/<rule>/<case>/case.yaml, sorted for determinism. */
export function discoverCases(specRoot: string): CaseFile[] {
  const fixtures = join(specRoot, "fixtures");
  const out: CaseFile[] = [];
  if (!isDir(fixtures)) return out;
  for (const rule of readdirSync(fixtures).sort()) {
    const ruleDir = join(fixtures, rule);
    if (!isDir(ruleDir)) continue;
    for (const c of readdirSync(ruleDir).sort()) {
      const dir = join(ruleDir, c);
      const file = join(dir, "case.yaml");
      if (!isDir(dir) || !isFile(file)) continue;
      const relPath = relative(specRoot, file).split("\\").join("/");
      try {
        out.push({ relPath, dir, data: loadYaml(file) });
      } catch (e) {
        out.push({ relPath, dir, data: undefined, parseError: (e as Error).message });
      }
    }
  }
  return out;
}

function isDir(p: string): boolean {
  try {
    return statSync(p).isDirectory();
  } catch {
    return false;
  }
}

function isFile(p: string): boolean {
  try {
    return statSync(p).isFile();
  } catch {
    return false;
  }
}
