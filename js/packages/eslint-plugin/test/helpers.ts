import { createRequire } from "node:module";
import { Linter, type Linter as L } from "eslint";
import * as tsParser from "@typescript-eslint/parser";
import plugin from "../src/index.js";

export const require = createRequire(import.meta.url);
export const FILE = "input.ts";

export type ParserName = "typescript-eslint" | "espree";

export function config(parser: ParserName, rules?: L.RulesRecord): L.Config[] {
  return [
    {
      files: ["**/*.ts"],
      plugins: { fldlint: plugin },
      languageOptions: parser === "espree" ? { sourceType: "module" } : { parser: tsParser, sourceType: "module" },
      rules: rules ?? (plugin.configs as { recommended: { rules: L.RulesRecord } }).recommended.rules,
    },
  ];
}

export const linter = (): Linter => new Linter({ configType: "flat" });

export function lint(code: string, parser: ParserName, rules?: L.RulesRecord): L.LintMessage[] {
  return linter().verify(code, config(parser, rules), FILE);
}

export function fix(code: string, parser: ParserName) {
  return linter().verifyAndFix(code, config(parser), FILE);
}

export const skipOf = (m: L.LintMessage): string | undefined => /\[skip: ([\w-]+)\]$/.exec(m.message)?.[1];
