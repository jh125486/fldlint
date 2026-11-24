import { lstatSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import type { RulesFile } from "./types.js";
import { decodeUtf8, parseSubset } from "./yamlSubset.js";

/** Read a spec YAML file, enforcing the fldlint YAML subset; throws YamlSubsetError on a violation. */
export function loadYaml(path: string): unknown {
  return parseSubset(decodeUtf8(readFileSync(path)));
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
  /** YAML parse or subset error message (`line L, col C: ...`), if any. */
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

const IGNORED_NAMES = new Set([".DS_Store", ".gitkeep"]);
const LANG_DIRS = new Set(["js", "go", "java"]);
const LANG_ALLOWED_FILES: Readonly<Record<string, readonly string[]>> = {
  js: ["input.ts", "output.ts"],
  go: ["input.go", "output.go.golden"],
  java: ["Before.java", "After.java"],
};

/**
 * Structural checks on spec/fixtures: layout must be exactly
 * fixtures/<rule>/<case>/case.yaml (+ js|go|java dirs holding known files).
 * Returns error messages (empty when the layout is sound).
 */
export function checkFixtureLayout(specRoot: string): string[] {
  const errors: string[] = [];
  const fixtures = join(specRoot, "fixtures");
  if (!isDir(fixtures)) return [`fixtures directory not found under ${specRoot}`];
  const rel = (p: string): string => relative(specRoot, p).split("\\").join("/");
  const entries = (d: string): string[] => readdirSync(d).filter((n) => !IGNORED_NAMES.has(n)).sort();
  for (const rule of entries(fixtures)) {
    const ruleDir = join(fixtures, rule);
    if (!isRealDir(ruleDir)) {
      errors.push(`${rel(ruleDir)}: stray file at fixtures/ level (expected only <rule> directories)`);
      continue;
    }
    for (const c of entries(ruleDir)) {
      const dir = join(ruleDir, c);
      if (!isRealDir(dir)) {
        errors.push(`${rel(dir)}: stray file at fixtures/<rule>/ level (expected only <case> directories)`);
        continue;
      }
      if (!isFile(join(dir, "case.yaml"))) {
        const alt = entries(dir).filter((n) => /^case\.ya?ml$/i.test(n) || /^case\./i.test(n));
        const hint = alt.length > 0 ? ` (found ${alt.join(", ")}; the file must be named exactly case.yaml)` : "";
        errors.push(`${rel(dir)}: missing case.yaml${hint}`);
      }
      for (const n of entries(dir)) {
        const p = join(dir, n);
        if (n === "case.yaml") continue;
        if (!isRealDir(p)) {
          errors.push(`${rel(p)}: unexpected file in case directory`);
        } else if (!LANG_DIRS.has(n)) {
          errors.push(`${rel(p)}: unexpected directory in case directory (only js, go, java allowed; wrong depth?)`);
        } else {
          const allowed = LANG_ALLOWED_FILES[n] ?? [];
          for (const f of entries(p)) {
            if (!allowed.includes(f)) {
              errors.push(`${rel(join(p, f))}: unexpected entry in ${n}/ (allowed: ${allowed.join(", ")})`);
            }
          }
        }
      }
    }
  }
  return errors;
}

function isRealDir(p: string): boolean {
  try {
    return lstatSync(p).isDirectory();
  } catch {
    return false;
  }
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
