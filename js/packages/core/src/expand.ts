import type { ArgRole, CallComment, CommentNotes } from "./callsite.js";

/** A line terminator as JS defines it; a comment holding one acts like a newline (ASI). */
export const hasLineTerminator = (s: string): boolean => new RegExp("[\\n\\r\\u2028\\u2029]").test(s);

/** Must not be followed directly by more code on its line: `//` comments and multi-line block comments. */
const breaks = (c: CallComment): boolean => c.line || hasLineTerminator(c.text);

const deferred = (cs: readonly CallComment[]): string => cs.map((c) => ` ${c.text}${c.line ? c.eol : ""}`).join("");

/**
 * Expand `{{name}}` placeholders in one pass (substituted text is never
 * re-expanded; an unknown placeholder stays as written) and place the call's
 * comments (issue #31). Returns `undefined` when a comment has no home.
 *
 * - An argument's `lead` comments go right before its text (a `//` comment
 *   is followed by its line terminator, a block comment by one space).
 * - Its `trail` comments that fit on the line go right after it. A `//` or
 *   multi-line one is moved behind the next `,` of the template (or, for the
 *   last argument, behind a comma added before the closing `)`), so the line
 *   terminator never splits an argument from its comma (Go needs this and it
 *   is valid JS).
 * - `head` comments go before the new call; `tail` comments before the final `)`.
 *
 * Decorations apply to a placeholder's first occurrence only.
 */
export function expand(template: string, values: Readonly<Record<string, string>>, notes?: CommentNotes): string | undefined {
  const tokens = template
    .split(/(\{\{\w+\}\})/)
    .filter((s) => s !== "")
    .map((s): { name: string } | { lit: string } => {
      const m = /^\{\{(\w+)\}\}$/.exec(s);
      return m?.[1] !== undefined && m[1] in values ? { name: m[1] } : { lit: s };
    });
  let lastPlaceholder = -1;
  tokens.forEach((t, i) => {
    if ("name" in t) lastPlaceholder = i;
  });

  let out = "";
  let pending: CallComment[] = [];
  const used = new Set<string>();
  for (const c of notes?.head ?? []) {
    if (breaks(c)) return undefined;
    out += `${c.text} `;
  }
  for (const [i, t] of tokens.entries()) {
    if ("lit" in t) {
      let lit = t.lit;
      if (pending.length > 0) {
        const comma = lit.indexOf(",");
        const close = lit.lastIndexOf(")");
        if (comma >= 0) {
          lit = `${lit.slice(0, comma + 1)}${deferred(pending)}${lit.slice(comma + 1)}`;
          pending = [];
        } else if (i === tokens.length - 1 && close >= 0) {
          lit = `${lit.slice(0, close)},${deferred(pending)}${lit.slice(close)}`;
          pending = [];
        }
      }
      out += lit;
      continue;
    }
    const role = t.name as ArgRole;
    const a = used.has(t.name) ? undefined : notes?.args[role];
    used.add(t.name);
    const trail = a?.trail ?? [];
    out += (a?.lead ?? []).map((c) => (c.line ? `${c.text}${c.eol}` : `${c.text} `)).join("");
    out += values[t.name];
    out += trail.filter((c) => !breaks(c)).map((c) => ` ${c.text}`).join("");
    pending.push(...trail.filter(breaks));
    if (i === lastPlaceholder) pending.push(...(notes?.tail ?? []));
  }
  if (pending.length > 0) return undefined;
  if ((notes?.tail.length ?? 0) > 0 && lastPlaceholder < 0) return undefined;
  // A comment bound to an argument the template never uses has no home.
  for (const [role, a] of Object.entries(notes?.args ?? {})) {
    if (!used.has(role) && (a.lead.length > 0 || a.trail.length > 0)) return undefined;
  }
  return out;
}
