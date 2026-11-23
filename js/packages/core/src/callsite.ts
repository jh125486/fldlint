import type { Edit } from "@fldlint/spec";

/** Outcome of the client-binding proof for one call site. */
export type ClientBinding =
  | { kind: "bound"; name: string }
  | { kind: "none" }
  | { kind: "ambiguous" };

/** Facts about the flag-default argument, derived syntactically. */
export type DefaultType = "boolean" | "string" | "number" | "object" | "unknown";

/** Facts about the context argument. `simple` means exactly `{ key, kind?: "user" }`. */
export type ContextShape = { kind: "simple"; keyText: string } | { kind: "other" };

/**
 * A detected LaunchDarkly call with a proven receiver, as plain data (no ESTree
 * nodes), so that another front end (the ast-grep codemod) can build one and
 * reuse {@link plan}.
 */
export interface CallSite {
  /** `rules.yaml` rule id the called method belongs to. */
  ruleId: string;
  /** Called method name, e.g. `boolVariation`. */
  method: string;
  /**
   * Half-open `[start, end)` UTF-16 range of the whole call expression, in the
   * coordinates of the text that was analyzed. `analyze()` reports them against
   * the source as given (a leading BOM counts as one code unit, so the range
   * can be passed to `applyEdits(source, ...)`); `line`/`col` follow ESLint and
   * ignore the BOM. Calling `detect()` directly (ESLint) uses the
   * BOM-stripped `sourceCode.text`.
   */
  range: [number, number];
  /** 1-based line of the call start (its callee object). */
  line: number;
  /** 1-based UTF-16 column of the call start; a tab counts as 1. */
  col: number;
  /** Flag key is a string literal or an expression-free template literal. */
  keyIsStatic: boolean;
  /** Verbatim source text of the key argument, quotes included (`{{key}}`). */
  keyText?: string;
  /**
   * Decoded string value of a static key; the fingerprint input. Absent when the
   * key is dynamic or not well formed (an unpaired surrogate).
   */
  keyValue?: string;
  /** Verbatim source text of the default argument (`{{default}}`). */
  defaultText?: string;
  defaultType: DefaultType;
  contextShape: ContextShape;
  clientBinding: ClientBinding;
}

export type { Edit };
