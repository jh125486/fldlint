export type Language = "js" | "go" | "java";
export const LANGUAGES: readonly Language[] = ["js", "go", "java"];

export type ValueType = "boolean" | "string" | "number" | "integer" | "object";
export type Risk = "low" | "medium" | "high";

export interface SkipCategory {
  severity: string;
  message: string;
}

export interface LanguageRule {
  methods: string[];
  template?: string;
  async?: boolean;
  ctxArg?: Record<string, number>;
  requires?: string[];
}

export interface Rule {
  id: string;
  valueType?: ValueType;
  risk: Risk;
  keyArg?: number;
  defaultArg?: number;
  contextArg?: number;
  skip?: string;
  /** Finding message for rewritable rules; placeholders limited to {{key}}. */
  message?: string;
  languages: Partial<Record<Language, LanguageRule>>;
}

export interface RulesFile {
  version: number;
  skipCategories: Record<string, SkipCategory>;
  rules: Rule[];
}
