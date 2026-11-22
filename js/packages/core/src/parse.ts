import { parseForESLint } from "@typescript-eslint/parser";
import type { TSESTree } from "@typescript-eslint/utils";
import type { ScopeManager, VisitorKeys } from "./detect.js";

export interface Parsed {
  ast: TSESTree.Program;
  scopeManager: ScopeManager;
  visitorKeys: VisitorKeys;
  /** The text that was parsed (leading BOM removed); all ranges index into it. */
  source: string;
}

export const BOM = "\uFEFF";

/** Parse JS/TS source the way the ESLint plugin sees it. `.tsx`/`.jsx` paths enable JSX. */
export function parseSource(input: string, filePath = "input.ts"): Parsed {
  // ESLint strips a leading BOM before parsing, so line-1 columns and ranges
  // only match if we do the same.
  const code = input.startsWith(BOM) ? input.slice(1) : input;
  const { ast, scopeManager, visitorKeys } = parseForESLint(code, {
    sourceType: "module",
    range: true,
    loc: true,
    comment: false,
    tokens: false,
    filePath,
    ecmaFeatures: { jsx: /\.[jt]sx$/.test(filePath) },
  });
  return {
    ast: ast as TSESTree.Program,
    scopeManager: scopeManager as unknown as ScopeManager,
    visitorKeys: visitorKeys as unknown as VisitorKeys,
    source: code,
  };
}
