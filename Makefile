.DEFAULT_GOAL := help
MAKEFLAGS += --silent --no-print-directory

APP_NAME := go-libyear
BIN_DIR := ./bin
MAIN_DIR := ./cmd/$(APP_NAME)
TEST_DIR := ./test

ifdef TERM
	BATS_FLAGS = --pretty
else
	BATS_FLAGS = -F tap
endif
ifeq (${BATS_DEBUG}, true)
	BATS_FLAGS += --trace --verbose-run
endif
BATS_BIN = $(TEST_DIR)/bats/bin/bats

ifndef VERSION
	VERSION := X.Y.Z
endif
ifndef GIT_TAG
	GIT_TAG := $(shell git rev-parse --short=8 HEAD)
endif
ifndef BUILD_DATE
	BUILD_DATE := $(shell git show -s --format=%cd --date=short $(GIT_TAG))
endif

LDFLAGS := "-s -w -X main.BuildVersion=$(VERSION) -X main.BuildGitTag=$(GIT_TAG) -X main.BuildDate=$(BUILD_DATE)"

# Print Makefile target step description for check.
# Only print 'check' steps this way, and not dependent steps, like 'install'.
# ${1} - step description
define _print_step
	printf -- '------\n%s...\n' "${1}"
endef

.PHONY: docker/build
## Build Docker image.
docker/build:
	docker build -t $(APP_NAME) --build-arg LDFLAGS=$(LDFLAGS) .

.PHONY: build
## Build the binary.
build:
	CGO_ENABLED=0 go build -ldflags=$(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) $(MAIN_DIR)

.PHONY: release
## Build and release the binaries.
release:
	@goreleaser release --snapshot --clean

.PHONY: test test/unit test/cli
## Run all tests.
test: test/unit test/cli

## Run cli bats tests.
test/cli:
	$(call _print_step,Running cli tests)
	$(eval VERSION := modified_value)
	docker build \
		--build-arg LDFLAGS="-X main.BuildVersion=2.0.0 -X main.BuildGitTag=v2.0.0 -X main.BuildDate=2023-10-23T08:03:03Z" \
		-t go-libyear-test-bin .
	docker build -t go-libyear-bats -f $(TEST_DIR)/Dockerfile .
	docker run --rm go-libyear-bats -F pretty $(TEST_DIR)/*

## Run all unit tests.
test/unit:
	$(call _print_step,Running unit tests)
	go test -race -cover ./...

.PHONY: check check/vet check/lint check/gosec check/spell check/trailing check/markdown check/format
## Run all checks.
check: check/vet check/lint check/gosec check/spell check/trailing check/markdown check/format

## Run 'go vet' on the whole project.
check/vet:
	$(call _print_step,Running go vet)
	go vet ./...

## Run golangci-lint all-in-one linter with configuration defined inside .golangci.yml.
check/lint:
	$(call _print_step,Running golangci-lint)
	golangci-lint run

## Check for security problems using gosec, which inspects the Go code by scanning the AST.
check/gosec:
	$(call _print_step,Running gosec)
	gosec -exclude-dir=test -exclude-generated -quiet ./...

## Check spelling, rules are defined in cspell.json.
check/spell:
	$(call _print_step,Verifying spelling)
	yarn --silent cspell --no-progress '**/**'

## Check for trailing whitespaces in any of the projects' files.
check/trailing:
	$(call _print_step,Looking for trailing whitespaces)
	./scripts/check-trailing-whitespaces.bash

## Check markdown files for potential issues with markdownlint.
check/markdown:
	$(call _print_step,Verifying Markdown files)
	yarn --silent markdownlint '**/*.md' --ignore node_modules

## Check for potential vulnerabilities across all Go dependencies.
check/vulns:
	$(call _print_step,Running govulncheck)
	govulncheck ./...

## Verify if the files are formatted.
## You must first commit the changes, otherwise it won't detect the diffs.
check/format:
	$(call _print_step,Checking if files are formatted)
	./scripts/check-formatting.sh

.PHONY: generate
## Generate Golang code.
generate:
	echo "Generating Go code..."
	go generate ./...

.PHONY: format format/go format/cspell
## Format files.
format: format/go format/cspell

## Format Go files.
format/go:
	echo "Formatting Go files..."
	gofumpt -l -w -extra .
	goimports -local=$$(head -1 go.mod | awk '{print $$2}') -w .
	golines -m 120 --ignore-generated --reformat-tags -w .

## Format cspell config file.
format/cspell:
	echo "Formatting cspell.yaml configuration (words list)..."
	yarn --silent format-cspell-config

.PHONY: install
## Install all dev dependencies.
install: install/yarn

## Install JS dependencies with yarn.
install/yarn:
	echo "Installing yarn dependencies..."
	yarn --silent install

.PHONY: help
## Print this help message.
help:
	./scripts/makefile-help.awk $(MAKEFILE_LIST)
