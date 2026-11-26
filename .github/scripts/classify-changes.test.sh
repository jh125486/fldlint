#!/usr/bin/env bash
# Table test for classify-changes.sh. Each case: name, newline-separated paths
# (printf %b escapes), expected "go js java docs_only".
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
fail=0

check() {
  local name=$1 paths=$2 want=$3 got
  got=$(printf '%b' "$paths" | "$here/classify-changes.sh" | sed 's/^[a-z_]*=//' | paste -sd' ' -)
  if [ "$got" = "$want" ]; then
    echo "ok   $name"
  else
    echo "FAIL $name: got '$got', want '$want'"
    fail=1
  fi
}

#     name                      paths                                               go    js    java  docs_only
check "go only"                 'go/internal/x/x.go\n'                              "true false false false"
check "js only"                 'js/packages/core/src/a.ts\njs/package-lock.json\n' "false true false false"
check "java only"               'java/pom.xml\njava/smoke/pom.xml\n'                "false false true false"
check "go and js"               'go/a.go\njs/b.ts\n'                                "true true false false"
check "go md stays go"          'go/README.md\n'                                    "true false false false"
check "go spec gen feeds java"  'go/internal/spec/gen/main.go\n'                    "true false true false"
check "spec change is all"      'spec/rules/x.yaml\n'                               "true true true false"
check "Taskfile is all"         'Taskfile.yml\n'                                    "true true true false"
check "workflow is all"         '.github/workflows/ci.yml\n'                        "true true true false"
check "action is all"           '.github/actions/foo/action.yml\n'                  "true true true false"
check "script is all"           '.github/scripts/classify-changes.sh\n'             "true true true false"
check "mise is all"             '.mise.toml\n'                                      "true true true false"
check "tool-versions is all"    '.tool-versions\n'                                  "true true true false"
check "go.work is all"          'go.work\n'                                         "true true true false"
check "root lockfile is all"    'package-lock.json\n'                               "true true true false"
check "docs dir only"           'docs/adr/009.md\ndocs/m0-contract.md\n'            "false false false true"
check "root md only"            'README.md\nCONTRIBUTING.md\n'                      "false false false true"
check "docs plus go"            'docs/a.md\ngo/a.go\n'                              "true false false false"
check "rename go to docs"       'go/a.go\ndocs/a.md\n'                              "true false false false"
check "rename docs to js"       'docs/a.ts\njs/a.ts\n'                              "false true false false"
check "rename across langs"     'go/a.go\njs/a.go\n'                                "true true false false"
check "rename within java"      'java/src/A.java\njava/src/B.java\n'                "false false true false"
check "docs-dir code file"      'docs/example.go\n'                                 "false false false true"
check "unknown root file"       'LICENSE\n'                                         "true true true false"
check "unknown path with docs"  'docs/a.md\nmystery/x.bin\n'                        "true true true false"
check "quoted path"             '"go/caf\\303\\251.go"\n'                           "true true true false"
check "empty list"              ''                                                  "true true true false"
check "blank lines only"        '\n\n'                                              "true true true false"
check "CRLF input"              'java/a.java\r\n'                                   "false false true false"
check "no trailing newline"     'js/a.ts'                                           "false true false false"

exit "$fail"
