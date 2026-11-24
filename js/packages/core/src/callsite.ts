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

/** One comment, verbatim (`text` includes the `//` or the slash-star delimiters). */
export interface CallComment {
  text: string;
  /** A `//` comment: it must be followed by a line terminator, `eol`. */
  line: boolean;
  /** The line terminator that ended a `//` comment in the source (`"\n"` or `"\r\n"`); empty for a block comment. */
  eol: string;
}

/** Template placeholders an argument can feed. */
export type ArgRole = "key" | "default" | "context";

/**
 * Where the comments of one call go in the rewrite (issue #31). Absent from a
 * {@link CallSite} when the call has no comment outside its copied argument text,
 * so a comment-free call plans exactly as before.
 *
 * - `args[role].lead`: comments that touch the argument from before it (own
 *   line above it, or on its line ahead of it). They move with it.
 * - `args[role].trail`: comments that follow it on the same line. They move with it.
 * - `head`: comments between the call start and its `(`; they go before the new call.
 * - `tail`: comments after the last argument that sit on their own line; they go
 *   before the closing `)`.
 * - `unhomed`: some comment has no home (inside the converted context literal,
 *   or a multi-line/line comment in `head`). The planner reports the call
 *   without a fix.
 */
export interface CommentNotes {
  args: Partial<Record<ArgRole, { lead: CallComment[]; trail: CallComment[] }>>;
  head: CallComment[];
  tail: CallComment[];
  unhomed: boolean;
}

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
  /** Comments inside the call range that the rewrite must place; see {@link CommentNotes}. */
  comments?: CommentNotes;
}

export type { Edit };
