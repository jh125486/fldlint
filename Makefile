# fldlint Makefile: the only entry points CI calls (see docs/m0-contract.md).
.DEFAULT_GOAL := help
SHELL := /bin/sh

MANDATORY_TAGS := quote-single quote-double typed-decl multi-declarator shadowed-client dynamic-key aliased-import
TAG_RULE := bool-variation
GEN_FILES := js/packages/spec/src/rules.gen.ts go/internal/spec/rules_gen.go

.PHONY: help spec-check spec-gen fixture-tags js-test go-test lint lint-go lint-js test

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

spec-check: ## Validate spec, regenerate code, fail on drift, check fixture tags
	cd js && npm ci && npm run spec:validate
	$(MAKE) spec-gen
	git diff --exit-code -- $(GEN_FILES)
	$(MAKE) fixture-tags

spec-gen: ## Regenerate JS and Go tables from spec/rules.yaml
	cd js && npm run gen -w @fldlint/spec
	cd go && go run ./internal/spec/gen

fixture-tags: ## Check mandatory shape tags are covered for bool-variation (ALLOW_EMPTY=1 tolerates missing fixtures)
	@dir=spec/fixtures/$(TAG_RULE); \
	if [ ! -d spec/fixtures ]; then \
	  if [ "$(ALLOW_EMPTY)" = "1" ]; then echo "fixture-tags: spec/fixtures does not exist yet; skipping (ALLOW_EMPTY=1)"; exit 0; fi; \
	  echo "fixture-tags: spec/fixtures does not exist (set ALLOW_EMPTY=1 to allow)" >&2; exit 1; \
	fi; \
	missing=0; \
	for tag in $(MANDATORY_TAGS); do \
	  if ! grep -qE "^[[:space:]]*tags:.*[[,[:space:]]$$tag[],[:space:]]" $$dir/*/case.yaml 2>/dev/null; then \
	    echo "fixture-tags: rule $(TAG_RULE) is missing mandatory tag: $$tag" >&2; missing=1; \
	  fi; \
	done; \
	if [ $$missing -ne 0 ]; then exit 1; fi; \
	echo "fixture-tags: all mandatory tags covered for $(TAG_RULE)"

js-test: ## Install and run JS tests
	cd js && npm ci && npm test

go-test: ## Run Go tests with the race detector
	cd go && go test -race ./...

lint: lint-go lint-js ## Run all linters

lint-go:
	cd go && golangci-lint run ./...

lint-js:
	@if [ -f js/package.json ] && grep -q '"lint"[[:space:]]*:' js/package.json; then \
	  cd js && npm ci && npm run lint; \
	else echo "lint-js: no npm lint script; skipping"; fi

test: js-test go-test ## Run all tests
