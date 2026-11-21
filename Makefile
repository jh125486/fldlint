# fldlint Makefile: the only entry points CI calls (see docs/m0-contract.md).
.DEFAULT_GOAL := help
SHELL := /bin/sh

GEN_FILES := js/packages/spec/src/rules.gen.ts go/internal/spec/rules_gen.go

.PHONY: help spec-check spec-gen fixture-tags js-test go-test lint lint-go lint-js test

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

spec-check: ## Validate spec, regenerate code, fail on drift, check fixture tags
	cd js && npm ci && npm run spec:validate
	$(MAKE) spec-gen
	@test -z "$$(git status --porcelain -- $(GEN_FILES))" || { \
	  echo "spec-check: generated files are out of date, run 'make spec-gen' and commit:" >&2; \
	  git status --porcelain -- $(GEN_FILES) >&2; exit 1; }
	$(MAKE) fixture-tags

spec-gen: ## Regenerate JS and Go tables from spec/rules.yaml
	cd js && npm run gen -w @fldlint/spec
	cd go && go run ./internal/spec/gen

fixture-tags: ## Check mandatory fixture tag coverage (ALLOW_EMPTY=1 tolerates missing fixtures)
	cd js && npm run spec:tags --silent$(if $(filter 1,$(ALLOW_EMPTY)), -- --allow-empty,)

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
