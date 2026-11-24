import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { ESLint } from "eslint";
import { require } from "./helpers.js";

// Real ESLint CLI and Node API against the built plugin (pretest builds dist).
const eslintBin = join(require.resolve("eslint/package.json"), "..", "bin", "eslint.js");
const pluginUrl = pathToFileURL(join(import.meta.dirname, "..", "dist", "index.js")).href;
const parserUrl = pathToFileURL(require.resolve("@typescript-eslint/parser")).href;

const SOURCE = `import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";
const ld = init("k");
const client = OpenFeature.getClient();
export const a = ld.boolVariation("a", { key: "u" }, false);
export const b = ld.allFlagsState({ key: "u" });
`;

// Each test spawns real ESLint processes, which can pass 5s on a loaded CI runner.
const CLI_TIMEOUT_MS = 30_000;

let dir: string;
const configText = (rules: string) => `import fldlint from ${JSON.stringify(pluginUrl)};
import * as parser from ${JSON.stringify(parserUrl)};
export default [
  fldlint.configs.recommended,
  { files: ["**/*.ts"], languageOptions: { parser } },
  { rules: ${rules} },
];
`;

function writeConfig(rules = "{}") {
  writeFileSync(join(dir, "eslint.config.mjs"), configText(rules));
}
function cli(...args: string[]) {
  return spawnSync(process.execPath, [eslintBin, ...args, "input.ts"], { cwd: dir, encoding: "utf8" });
}

beforeAll(() => {
  dir = mkdtempSync(join(tmpdir(), "fldlint-eslint-"));
  writeFileSync(join(dir, "input.ts"), SOURCE);
});
afterAll(() => rmSync(dir, { recursive: true, force: true }));

describe("ESLint CLI exit codes", { timeout: CLI_TIMEOUT_MS }, () => {
  it("recommended warns and exits 0 by default (max-warnings -1)", () => {
    writeConfig();
    const r = cli();
    expect(r.status).toBe(0);
    expect(r.stdout).toContain("fldlint/bool-variation");
    expect(r.stdout).toContain("[skip: bulk]");
    expect(r.stdout).toContain("2 warnings");
  });

  it("--max-warnings 0 fails with 2 warnings; --max-warnings 2 passes; 1 fails", () => {
    writeConfig();
    expect(cli("--max-warnings", "0").status).toBe(1);
    expect(cli("--max-warnings", "1").status).toBe(1);
    expect(cli("--max-warnings", "2").status).toBe(0);
  });

  it("an error-level rule exits 1; off removes it", () => {
    writeConfig(`{ "fldlint/bool-variation": "error" }`);
    expect(cli().status).toBe(1);
    writeConfig(`{ "fldlint/bool-variation": "off", "fldlint/all-flags-state": "off" }`);
    const r = cli("--max-warnings", "0");
    expect(r.status).toBe(0);
    expect(r.stdout).toBe("");
  });

  it("--quiet hides warnings and never changes the exit code", () => {
    writeConfig();
    const quiet = cli("--quiet", "--max-warnings", "0");
    expect(quiet.stdout).toBe("");
    expect(quiet.status).toBe(1);
    expect(cli("--quiet").status).toBe(0);
  });

  it("a configuration error exits 2", () => {
    writeConfig(`{ "fldlint/no-such-rule": "warn" }`);
    expect(cli().status).toBe(2);
  });

  it("--format json reports positions and severity", () => {
    writeConfig();
    const [out] = JSON.parse(cli("-f", "json").stdout) as [{ warningCount: number; messages: unknown[] }];
    expect(out.warningCount).toBe(2);
    expect(out.messages[0]).toMatchObject({ ruleId: "fldlint/bool-variation", severity: 1, line: 5, column: 18 });
  });
});

describe("ESLint CLI --fix", { timeout: CLI_TIMEOUT_MS }, () => {
  it("--fix-dry-run does not write; --fix writes and exits on what remains", () => {
    writeConfig();
    const dry = cli("--fix-dry-run", "-f", "json");
    const [result] = JSON.parse(dry.stdout) as [{ output: string }];
    expect(result.output).toContain("client.getBooleanValue");
    expect(readFileSync(join(dir, "input.ts"), "utf8")).toBe(SOURCE);

    const fixed = cli("--fix", "--max-warnings", "0");
    expect(readFileSync(join(dir, "input.ts"), "utf8")).toContain(
      `client.getBooleanValue("a", false, { targetingKey: "u" })`,
    );
    // The bulk call remains (1 warning), so max-warnings 0 still fails.
    expect(fixed.status).toBe(1);
    expect(fixed.stdout).toContain("1 warning");
  });
});

describe("ESLint Node API", { timeout: CLI_TIMEOUT_MS }, () => {
  it("applies per-rule severity from flat config", async () => {
    writeFileSync(join(dir, "input.ts"), SOURCE);
    writeConfig(`{ "fldlint/all-flags-state": "error" }`);
    const eslint = new ESLint({ cwd: dir, overrideConfigFile: join(dir, "eslint.config.mjs") });
    const [res] = await eslint.lintFiles(["input.ts"]);
    expect(res).toMatchObject({ errorCount: 1, warningCount: 1, fixableWarningCount: 1, fixableErrorCount: 0 });
  });
});
