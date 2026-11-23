// SPDX-License-Identifier: MIT
import { createRequire } from "node:module";
import type { ESLint, Linter, Rule } from "eslint";
import { skipCategories } from "@fldlint/spec";
import { makeRules } from "./rules.js";

const pkg = createRequire(import.meta.url)("../package.json") as { name: string; version: string };

/** Rule modules keyed by `rules.yaml` rule id (only rules with a JS method list). */
export const rules: Record<string, Rule.RuleModule> = makeRules();

const plugin: ESLint.Plugin = {
  meta: { name: pkg.name, version: pkg.version, namespace: "fldlint" },
  rules,
  configs: {},
};

/** Every fldlint rule at `warn` (ADR 008). No parser is set: bring your own for TypeScript. */
const recommended: Linter.Config = {
  name: "fldlint/recommended",
  plugins: { fldlint: plugin },
  rules: Object.fromEntries(Object.keys(rules).map((id) => [`fldlint/${id}`, "warn" as const])),
};

export const configs: { recommended: Linter.Config } = { recommended };
Object.assign(plugin.configs as object, configs);

export const meta = plugin.meta as NonNullable<ESLint.Plugin["meta"]>;
export const skipCategoryIds: readonly string[] = Object.keys(skipCategories);
export default plugin;
