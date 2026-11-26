#!/usr/bin/env bash
# Table test for classify-changes.sh. Each case: name, newline-separated paths
# (printf %b escapes), expected "go js java shared".
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
check "go only"                 'go/internal/x/x.go\n'                              "true false false true"
check "js only"                 'js/packages/core/src/a.ts\njs/package-lock.json\n' "false true false true"
check "java only"               'java/pom.xml\njava/smoke/pom.xml\n'                "false false true true"
check "go and js"               'go/a.go\njs/b.ts\n'                                "true true false true"
check "go md stays go"          'go/README.md\n'                                    "true false false true"
check "go spec gen feeds java"  'go/internal/spec/gen/main.go\n'                    "true false true true"
check "spec change is all"      'spec/rules/x.yaml\n'                               "true true true true"
check "Taskfile is all"         'Taskfile.yml\n'                                    "true true true true"
check "workflow is all"         '.github/workflows/ci.yml\n'                        "true true true true"
check "action is all"           '.github/actions/foo/action.yml\n'                  "true true true true"
check "script is all"           '.github/scripts/classify-changes.sh\n'             "true true true true"
check "mise is all"             '.mise.toml\n'                                      "true true true true"
check "tool-versions is all"    '.tool-versions\n'                                  "true true true true"
check "go.work is all"          'go.work\n'                                         "true true true true"
check "root lockfile is all"    'package-lock.json\n'                               "true true true true"
check "docs dir only"           'docs/adr/009.md\ndocs/m0-contract.md\n'            "false false false false"
check "root md only"            'README.md\nCONTRIBUTING.md\n'                      "false false false false"
check "docs plus go"            'docs/a.md\ngo/a.go\n'                              "true false false true"
check "rename go to docs"       'go/a.go\ndocs/a.md\n'                              "true false false true"
check "rename docs to js"       'docs/a.ts\njs/a.ts\n'                              "false true false true"
check "rename across langs"     'go/a.go\njs/a.go\n'                                "true true false true"
check "rename within java"      'java/src/A.java\njava/src/B.java\n'                "false false true true"
check "docs-dir code file"      'docs/example.go\n'                                 "false false false false"
check "unknown root file"       'LICENSE\n'                                         "true true true true"
check "unknown path with docs"  'docs/a.md\nmystery/x.bin\n'                        "true true true true"
check "quoted path"             '"go/caf\\303\\251.go"\n'                           "true true true true"
check "empty list"              ''                                                  "true true true true"
check "blank lines only"        '\n\n'                                              "true true true true"
check "CRLF input"              'java/a.java\r\n'                                   "false false true true"
check "no trailing newline"     'js/a.ts'                                           "false true false true"
check "gen RulesGen.java"       'java/src/main/java/io/github/jh125486/fldlint/spec/RulesGen.java\n' "true true true true"
check "gen rules.gen.ts"        'js/packages/spec/src/rules.gen.ts\n'              "true true true true"
check "gen rules_gen.go"        'go/internal/spec/rules_gen.go\n'                  "true true true true"
check "gen spec.json"           'spec/generated/spec.json\n'                       "true true true true"
check "gen go testdata spec"    'go/internal/spec/testdata/spec.json\n'            "true true true true"
check "js spec package"         'js/packages/spec/src/bin/gen.ts\n'                "true true true true"
check "conformance fixtures"    'go/conformance/testdata/fixtures/a.yaml\n'        "true true true true"
check "path with spaces"        'go/my dir/a b.go\n'                               "true false false true"
check "near-miss specx"         'go/internal/specx/a.go\n'                         "true false false true"
check ".github README"          '.github/README.md\n'                              "true true true true"
check ".gitattributes"          '.gitattributes\n'                                 "true true true true"
check "quoted non-ASCII docs"   '"docs/caf\\303\\251.md"\n'                         "true true true true"

exit "$fail"
