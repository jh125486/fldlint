// Runs the generated cases through the real Codemod runtime (QuickJS + LLRT).
import { spawnSync } from "node:child_process";
import { join } from "node:path";

const pkg = new URL("..", import.meta.url).pathname;
const transform = join(pkg, "dist", "transform.js");
for (const [lang, dir] of [["typescript", ".jssg-tests"], ["tsx", ".jssg-tests-tsx"]]) {
  const r = spawnSync("npx", ["--yes", "codemod@1.18.7", "jssg", "test", "-l", lang, transform, join(pkg, dir)], {
    stdio: "inherit",
  });
  if (r.status !== 0) process.exit(r.status ?? 1);
}
