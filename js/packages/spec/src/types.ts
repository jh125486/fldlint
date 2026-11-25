export type Language = "js" | "go" | "java";
export const LANGUAGES: readonly Language[] = ["js", "go", "java"];

export type ValueType = "boolean" | "string" | "number" | "integer" | "object";
export type Risk = "low" | "medium" | "high";

export interface SkipCategory {
  severity: string;
  message: string;
}

/** Inclusive argument-count range of an SDK method; `max` is absent for a variadic method. */
export interface ArgCount {
  min: number;
  max?: number;
}

export interface LanguageRule {
  methods: string[];
  template?: string;
  async?: boolean;
  ctxArg?: Record<string, number>;
  /** Report-only rules: accepted argument count of the non-Ctx form (Go ctxArg methods add one). */
  argCount?: ArgCount;
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
