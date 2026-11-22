import { parseForESLint } from "@typescript-eslint/parser";
import type { TSESTree } from "@typescript-eslint/utils";
import type { ScopeManager } from "./detect.js";

export interface Parsed {
  ast: TSESTree.Program;
  scopeManager: ScopeManager;
}

/** Parse JS/TS source the way the ESLint plugin sees it. `.tsx`/`.jsx` paths enable JSX. */
export function parseSource(code: string, filePath = "input.ts"): Parsed {
  const { ast, scopeManager } = parseForESLint(code, {
    sourceType: "module",
    range: true,
    loc: true,
    comment: false,
    tokens: false,
    filePath,
    ecmaFeatures: { jsx: /\.[jt]sx$/.test(filePath) },
  });
  return { ast: ast as TSESTree.Program, scopeManager: scopeManager as unknown as ScopeManager };
}
