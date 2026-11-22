#!/usr/bin/env node
// Parity helper: recompute every fingerprint in a vectors file.
//   fp.js [vectors.json]               prints one fingerprint per line
//   fp.js --baseline out.json [vectors.json]   writes a baseline from them
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { writeBaseline } from "../baseline.js";
import { fingerprint, type FingerprintInput } from "../fingerprint.js";

const here = dirname(fileURLToPath(import.meta.url));
const args = process.argv.slice(2);
let baselineOut: string | undefined;
if (args[0] === "--baseline") {
  baselineOut = args[1];
  if (!baselineOut) {
    console.error("usage: fp.js [--baseline out.json] [vectors.json]");
    process.exit(2);
  }
  args.splice(0, 2);
}
const vectorsPath = args[0] ?? resolve(here, "../../../../../spec/contracts/fingerprint-vectors.json");
const vectors = JSON.parse(readFileSync(vectorsPath, "utf8")) as FingerprintInput[];
const fps = vectors.map(fingerprint);
if (baselineOut) writeBaseline(baselineOut, fps);
else process.stdout.write(fps.map((f) => f + "\n").join(""));
