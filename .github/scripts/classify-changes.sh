#!/usr/bin/env bash
# Classify a list of changed paths (one per line, stdin) into CI scope flags.
# Prints go=, js=, java=, shared= and workflows= lines (true/false), ready to
# append to $GITHUB_OUTPUT. Fails safe: an empty list or any unrecognized path
# turns on every language. shared=true means any non-documentation change
# (spec:check, the workflow tests and parity are language-independent and run
# on it). workflows=true means a workflow-lint input changed (.github/**,
# Taskfile.yml, go/go.mod, go/go.sum) or the list is empty.
# Used by the `changes` job in ci.yml; tested by classify-changes.test.sh
# (task workflows:test).
#
# Feed it `git -c core.quotePath=false diff --no-renames --name-only`: a rename
# is then a delete plus an add, so both the old and the new location count, and
# non-ASCII paths arrive unquoted.
set -euo pipefail

go=false js=false java=false workflows=false n=0 nondocs=0
all() { go=true js=true java=true; }

while IFS= read -r p || [ -n "$p" ]; do
  p=${p%$'\r'}
  [ -n "$p" ] || continue
  n=$((n + 1))
  case $p in .github/* | Taskfile.yml | go/go.mod | go/go.sum) workflows=true ;; esac
  case $p in
    # Generated files (Taskfile GEN_FILES), shared fixtures and the spec generator:
    # every language consumes them, and spec:check must see them.
    js/packages/spec/* | go/conformance/testdata/* | spec/generated/spec.json \
      | go/internal/spec/rules_gen.go | go/internal/spec/testdata/spec.json \
      | java/src/main/java/io/github/jh125486/fldlint/spec/RulesGen.java) all ;;
    # The Java rule table is generated from the spec by go/internal/spec/gen.
    go/internal/spec/*) go=true java=true ;;
    go/*) go=true ;;
    js/*) js=true ;;
    java/*) java=true ;;
    # Shared inputs: spec, task runner, workflows, tool versions, root manifests.
    spec/* | Taskfile.yml | .github/* | .mise.toml | .tool-versions | go.work | go.work.sum \
      | package.json | package-lock.json | pnpm-lock.yaml | pnpm-workspace.yaml | yarn.lock \
      | pom.xml | build.gradle | build.gradle.kts | settings.gradle | settings.gradle.kts) all ;;
    # Documentation outside the code trees.
    docs/* | *.md) continue ;;
    # Unknown: run everything.
    *) all ;;
  esac
  nondocs=$((nondocs + 1))
done

# Empty input: run everything.
if [ "$n" -eq 0 ]; then all; workflows=true; fi

shared=true
if [ "$n" -gt 0 ] && [ "$nondocs" -eq 0 ]; then shared=false; fi

printf 'go=%s\njs=%s\njava=%s\nshared=%s\nworkflows=%s\n' "$go" "$js" "$java" "$shared" "$workflows"
