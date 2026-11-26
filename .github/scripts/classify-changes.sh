#!/usr/bin/env bash
# Classify a list of changed paths (one per line, stdin) into CI scope flags.
# Prints go=, js=, java= and docs_only= lines (true/false), ready to append to
# $GITHUB_OUTPUT. Fails safe: an empty list or any unrecognized path turns on
# every language. Used by the `changes` job in ci.yml; tested by
# classify-changes.test.sh (task workflows:test).
#
# Feed it `git diff --no-renames --name-only`: a rename is then a delete plus an
# add, so both the old and the new location count.
set -euo pipefail

go=false js=false java=false n=0 nondocs=0
all() { go=true js=true java=true; }

while IFS= read -r p || [ -n "$p" ]; do
  p=${p%$'\r'}
  [ -n "$p" ] || continue
  n=$((n + 1))
  case $p in
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
if [ "$n" -eq 0 ]; then all; fi

docs_only=false
if [ "$n" -gt 0 ] && [ "$nondocs" -eq 0 ]; then docs_only=true; fi

printf 'go=%s\njs=%s\njava=%s\ndocs_only=%s\n' "$go" "$js" "$java" "$docs_only"
