.DEFAULT_GOAL := help
MAKEFLAGS += --silent --no-print-directory

APP_NAME := go-libyear
BIN_DIR := ./bin
MAIN_DIR := ./cmd
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

# renovate datasource=github-releases depName=securego/gosec
GOSEC_VERSION := v2.18.2
# renovate datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION := v1.55.2
# renovate datasource=go depName=golang.org/x/vuln/cmd/govulncheck
GOVULNCHECK_VERSION := v1.0.1
# renovate datasource=go depName=golang.org/x/tools/cmd/goimports
GOIMPORTS_VERSION := v0.16.0

# Check if the program is present in $PATH and install otherwise.
# ${1} - oneOf{binary,yarn}
# ${2} - program name
define _ensure_installed
	LOCAL_BIN_DIR=$(BIN_DIR) ./scripts/ensure_installed.sh "${1}" "${2}"
endef

# Install Go binary using 'go install' with an output directory set via $GOBIN.
# ${1} - repository url
define _install_go_binary
	GOBIN=$(realpath $(BIN_DIR)) go install "${1}"
endef

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
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) $(MAIN_DIR)

.PHONY: release
## Build and release the binaries.
release:
	@goreleaser release --snapshot --clean

.PHONY: test test/unit test/cli
## Run all tests.
test: test/unit test/cli

## Run cli bats tests.
test/cli: test/cli/init
	$(call _print_step,Running cli tests)
	$(eval VERSION := modified_value)
	docker build \
		--build-arg LDFLAGS="-X main.BuildVersion=2.0.0 -X main.BuildGitTag=v2.0.0 -X main.BuildDate=2023-10-23T08:03:03Z" \
		-t go-libyear-test-bin .
	docker build -t go-libyear-bats -f $(TEST_DIR)/Dockerfile .
	docker run --rm go-libyear-bats $(TEST_DIR)/*

## Initialize bats tests framework.
test/cli/init:
	git submodule update --init --recursive $(TEST_DIR)

## Run all unit tests.
test/unit:
	$(call _print_step,Running unit tests)
	go test -race -cover ./...

.PHONY: check check/vet check/lint check/gosec check/spell check/trailing check/markdown check/format check/vulns
## Run all checks.
check: check/vet check/lint check/gosec check/spell check/trailing check/markdown check/format check/vulns

## Run 'go vet' on the whole project.
check/vet:
	$(call _print_step,Running go vet)
	go vet ./...

## Run golangci-lint all-in-one linter with configuration defined inside .golangci.yml.
check/lint:
	$(call _print_step,Running golangci-lint)
	$(call _ensure_installed,binary,golangci-lint)
	$(BIN_DIR)/golangci-lint run

## Check for security problems using gosec, which inspects the Go code by scanning the AST.
check/gosec:
	$(call _print_step,Running gosec)
	$(call _ensure_installed,binary,gosec)
	$(BIN_DIR)/gosec -exclude-dir=test -exclude-generated -quiet ./...

## Check spelling, rules are defined in cspell.json.
check/spell:
	$(call _print_step,Verifying spelling)
	$(call _ensure_installed,yarn,cspell)
	yarn --silent cspell --no-progress '**/**'

## Check for trailing whitespaces in any of the projects' files.
check/trailing:
	$(call _print_step,Looking for trailing whitespaces)
	./scripts/check-trailing-whitespaces.bash

## Check markdown files for potential issues with markdownlint.
check/markdown:
	$(call _print_step,Verifying Markdown files)
	$(call _ensure_installed,yarn,markdownlint)
	yarn --silent markdownlint '*.md' --disable MD010, MD034 # MD010 does not handle code blocks well.

## Check for potential vulnerabilities across all Go dependencies.
check/vulns:
	$(call _print_step,Running govulncheck)
	$(call _ensure_installed,binary,govulncheck)
	$(BIN_DIR)/govulncheck ./...

## Verify if the files are formatted.
## You must first commit the changes, otherwise it won't detect the diffs.
check/format:
	$(call _print_step,Checking if files are formatted)
	./scripts/check-formatting.sh

.PHONY: generate
## Generate Golang code.
generate:
	echo "Generating Go code..."
	#$(call _ensure_installed,binary,go-enum)
	go generate ./...

.PHONY: format format/go format/cspell
## Format files.
format: format/go format/cspell

## Format Go files.
format/go:
	echo "Formatting Go files..."
	$(call _ensure_installed,binary,goimports)
	go fmt ./...
	$(BIN_DIR)/goimports -local=$$(head -1 go.mod | awk '{print $$2}') -w .

## Format cspell config file.
format/cspell:
	echo "Formatting cspell.yaml configuration (words list)..."
	$(call _ensure_installed,yarn,yaml)
	yarn --silent format-cspell-config

.PHONY: install install/yarn install/golangci-lint install/gosec install/govulncheck install/goimports
## Install all dev dependencies.
install: install/yarn install/golangci-lint install/gosec install/govulncheck install/goimports

## Install JS dependencies with yarn.
install/yarn:
	echo "Installing yarn dependencies..."
	yarn --silent install

## Install golangci-lint (https://golangci-lint.run).
install/golangci-lint:
	echo "Installing golangci-lint..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh |\
 		sh -s -- -b $(BIN_DIR) $(GOLANGCI_LINT_VERSION)

## Install gosec (https://github.com/securego/gosec).
install/gosec:
	echo "Installing gosec..."
	curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh |\
 		sh -s -- -b $(BIN_DIR) $(GOSEC_VERSION)

## Install govulncheck (https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck).
install/govulncheck:
	echo "Installing govulncheck..."
	$(call _install_go_binary,golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION))

## Install goimports (https://pkg.go.dev/golang.org/x/tools/cmd/goimports).
install/goimports:
	echo "Installing goimports..."
	$(call _install_go_binary,golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION))

.PHONY: help
## Print this help message.
help:
	./scripts/makefile-help.awk $(MAKEFILE_LIST)
