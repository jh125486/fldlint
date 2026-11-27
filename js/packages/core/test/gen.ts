// Seeded program generator for the property tests (issue #21, part 1). The
// grammar mirrors the Go planner fuzzer (go/conformance/fuzz_test.go):
// receiver kind x key shape x context shape x client-binding shape x
// default shape x method x comment shape, plus shadowing of the OpenFeature
// client name. No dependency: a mulberry32 generator, so a failing seed
// reproduces exactly.

/** mulberry32: a tiny, well-distributed 32-bit PRNG. Returns floats in [0, 1). */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export class Rng {
  private readonly next: () => number;
  constructor(readonly seed: number) {
    this.next = mulberry32(seed);
  }
  /** Uniform integer in [0, n). */
  int(n: number): number {
    return Math.floor(this.next() * n);
  }
  bool(): boolean {
    return this.int(2) === 1;
  }
  pick<T>(xs: readonly T[]): T {
    const x = xs[this.int(xs.length)];
    if (x === undefined) throw new Error("pick from an empty list");
    return x;
  }
}

interface Method {
  name: string;
  /** A default argument of the matching type (the rule's value type). */
  def: string;
  /** A default of another type: the type is not provable for this rule. */
  wrongDef: string;
  keyless?: boolean;
}

const METHODS: readonly Method[] = [
  { name: "boolVariation", def: "false", wrongDef: `"x"` },
  { name: "stringVariation", def: `"dv"`, wrongDef: "1" },
  { name: "numberVariation", def: "1.5", wrongDef: "false" },
  { name: "jsonVariation", def: `{ a: 1 }`, wrongDef: "1" },
  { name: "variation", def: "false", wrongDef: `"x"` },
  { name: "boolVariationDetail", def: "false", wrongDef: `"x"` },
  { name: "allFlagsState", def: "", wrongDef: "", keyless: true },
];

const KEY_ALPHABET = ["a", "b", "-", "_", ".", "0", "é", "中", "\u{1F600}", " ", "\\u0041", "\\n"];

/** One generated program: its text and the choices that made it. */
export interface Generated {
  seed: number;
  source: string;
  choices: Record<string, string | number | boolean>;
}

/** Generate the program for `seed`. Pure: the same seed gives the same text. */
export function generate(seed: number): Generated {
  const r = new Rng(seed);
  const method = r.pick(METHODS);
  // Repeated entries weight the draw toward shapes that can be rewritten, so
  // the interesting skip orderings are reached from a proven receiver.
  const recv = r.pick([0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5]);
  const key = r.pick([0, 0, 1, 2, 2, 7, 7, 3, 4, 5, 6, 8]);
  const ctx = r.pick([0, 0, 0, 1, 1, 1, 2, 3, 4, 5, 6]);
  const bind = r.pick([1, 1, 1, 1, 1, 2, 2, 0, 3, 4, 5]);
  const comment = r.pick([0, 0, 0, 0, 0, 1, 2, 3, 4]);
  const badDefault = r.int(6) === 0;
  const two = r.bool();
  const quote = r.pick(['"', "'"]);
  const awaited = r.int(5) !== 0;
  // A same-line prefix with a 2-byte and an astral character before the call,
  // so the reported column differs between UTF-16 units, code points and bytes.
  const uniPrefix = r.int(3) === 0;

  const q = (s: string) => `${quote}${s}${quote}`;
  const header: string[] = [`import { init } from ${q("@launchdarkly/node-server-sdk")};`];
  if (bind !== 0) header.push(`import { OpenFeature } from ${q("@openfeature/server-sdk")};`);

  // Receiver: how the LaunchDarkly client is bound and referred to.
  const top: string[] = [];
  const params: string[] = ["u: string", "k: string", "dflt: unknown", "c: { key: string }"];
  let recvExpr = "ld";
  switch (recv) {
    case 0:
      top.push(`const ld = init(${q("sdk-key")});`);
      break;
    case 1:
      top.push(`let ld = init(${q("sdk-key")});`, `ld = init(${q("other-key")});`);
      break;
    case 2:
      top.push(`const holder = { ld: init(${q("sdk-key")}) };`);
      recvExpr = "holder.ld";
      break;
    case 3:
      top.push(`function getLd() { return init(${q("sdk-key")}); }`);
      recvExpr = "getLd()";
      break;
    case 4:
      // The parameter shadows nothing and is not an SDK client: no finding.
      params.unshift("ld: any");
      break;
    default:
      top.push(`const ld = init(${q("sdk-key")});`);
      params.unshift("ld: any"); // shadows the module-level client
  }

  // Client binding for the OpenFeature side.
  const body: string[] = [];
  switch (bind) {
    case 1:
      top.push("const client = OpenFeature.getClient();");
      break;
    case 2:
      body.push("const client = OpenFeature.getClient();");
      break;
    case 3:
      top.push("const client = OpenFeature.getClient();", `const billing = OpenFeature.getClient(${q("billing")});`);
      break;
    case 4:
      top.push("const client = OpenFeature.getClient();");
      params.push("client: unknown"); // shadows the module-level client
      break;
    case 5:
      top.push("let client = OpenFeature.getClient();", "client = OpenFeature.getClient();");
      break;
  }

  // Flag key.
  let rnd = "";
  for (let i = r.int(4) + 1; i > 0; i--) rnd += r.pick(KEY_ALPHABET);
  const staticKey = `${quote}${rnd}${quote}`;
  const keyExpr = [
    staticKey,
    q("flag-key"),
    "`tpl-key`",
    "`tpl-${k}`",
    "k",
    `${q("a")} + ${q("b")}`,
    `${quote}\\uD800${quote}`, // lone surrogate: not well formed
    `${quote}café \u{1F600}${quote}`,
    "[k][0]",
  ][key] as string;

  // Context argument.
  const ctxExpr = [
    "{ key: u }",
    `{ kind: ${q("user")}, key: u }`,
    "{ key: u, name: 1 }",
    "c",
    "{ ...c }",
    `{ kind: ${q("org")}, key: u }`,
    "{ key: `${u}-x` }",
  ][ctx] as string;

  const def = badDefault ? method.wrongDef || "dflt" : method.def;
  const mkCall = (kExpr: string, cExpr: string) => {
    const args = method.keyless ? [cExpr] : [kExpr, cExpr, def];
    const aw = awaited ? "await " : "";
    const inner = args.join(", ");
    switch (comment) {
      case 2:
        return `${aw}${recvExpr}.${method.name}(\n    ${args[0]}${args.length > 1 ? "," : ""} // trailing\n    ${args.slice(1).join(", ")}\n  )`;
      case 3:
        return `${aw}${recvExpr}. /* c */ ${method.name}(${inner})`;
      case 4:
        return `${aw}${recvExpr}.${method.name}(${args[0]}${args.length > 1 ? "," : ""} /* mid */ ${args.slice(1).join(", ")})`;
      case 1:
        return `${aw}${recvExpr}.${method.name}(/* before */ ${inner})`;
      default:
        return `${aw}${recvExpr}.${method.name}(${inner})`;
    }
  };

  const lines = [`export async function f(${params.join(", ")}) {`];
  for (const b of body) lines.push(`  ${b}`);
  lines.push(`  const ${uniPrefix ? `label = ${q("caf\u00e9 \u{1F600}")}, ` : ""}v = ${mkCall(keyExpr, ctxExpr)};`);
  if (two) lines.push(`  const w = ${mkCall(q("second"), "{ key: u, extra: true }")};`, "  return [v, w];");
  else lines.push("  return v;");
  lines.push("}");

  const source = [...header, "", ...top, "", ...lines, ""].join("\n");
  return {
    seed,
    source,
    choices: { method: method.name, recv, key, ctx, bind, comment, badDefault, two, awaited, uniPrefix },
  };
}

/** A seeded pseudo-random path in the shapes `normalizePath` has to handle. */
export function generatePath(seed: number): string {
  const r = new Rng(seed);
  const parts = ["a", "b", ".", "..", "", "é", "é", "\u{1F600}", "src", "x.ts"];
  const seps = ["/", "\\", "//", "/./", "\\.\\", "/"];
  let s = r.bool() ? r.pick(["./", ".\\", "/", ""]) : "";
  for (let i = r.int(6) + 1; i > 0; i--) s += r.pick(parts) + r.pick(seps);
  if (r.bool()) s = s.slice(0, -1);
  return s;
}
