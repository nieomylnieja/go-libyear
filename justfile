set shell := ["bash", "-c"]

bin_dir := "./bin"
scripts_dir := "./scripts"
app_name := "go-libyear"
main_dir := "./cmd/go-libyear"
test_dir := "./test"
print_step := 'printf -- "------\n%s...\n"'
build_version := env_var_or_default("VERSION", "X.Y.Z")
build_git_tag := if env_var_or_default("GIT_TAG", "") != "" { env_var("GIT_TAG") } else { shell("git rev-parse --short=8 HEAD") }
build_date := if env_var_or_default("BUILD_DATE", "") != "" { env_var("BUILD_DATE") } else { shell("git show -s --format=%cd --date=short \"$1\"", build_git_tag) }
build_ldflags := "-s -w -X main.BuildVersion=" + build_version + " -X main.BuildGitTag=" + build_git_tag + " -X main.BuildDate=" + build_date

# Print this help message
[private]
default:
    @just --list

# Activate developer environment using devbox, run `just install-devbox` first if you don't have devbox installed
activate:
    devbox shell

# Install devbox binary
install-devbox:
    @{{ print_step }} "Installing devbox"
    curl -fsSL https://get.jetpack.io/devbox | bash

# Update devbox managed package versions
update-devbox:
    @{{ print_step }} "Update packages managed by devbox"
    devbox update

# Build Docker image
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    {{ print_step }} "Building Docker image"
    docker build -t {{ app_name }} --build-arg "LDFLAGS={{ build_ldflags }}" .

# Build go-libyear binary
build:
    #!/usr/bin/env bash
    set -euo pipefail
    {{ print_step }} "Building binary"
    mkdir -p {{ bin_dir }}
    CGO_ENABLED=0 go build -ldflags="{{ build_ldflags }}" -o {{ bin_dir }}/{{ app_name }} {{ main_dir }}

# Install go-libyear binary
install:
    #!/usr/bin/env bash
    set -euo pipefail
    {{ print_step }} "Installing binary"
    go install -ldflags="{{ build_ldflags }}" ./cmd/{{ app_name }}

# Build and release the binaries
release:
    @{{ print_step }} "Releasing binary"
    goreleaser release --snapshot --clean

# Run all tests
test: test-unit test-cli

# Run all unit tests
test-unit:
    @{{ print_step }} "Running unit tests"
    go test -race -cover ./...

# Run CLI bats tests
test-cli:
    #!/usr/bin/env bash
    set -euo pipefail
    {{ print_step }} "Running CLI tests"
    docker build \
      --build-arg "LDFLAGS=-X main.BuildVersion=2.0.0 -X main.BuildGitTag=v2.0.0 -X main.BuildDate=2023-10-23T08:03:03Z" \
      -t go-libyear-test-bin .
    docker build -t go-libyear-bats -f {{ test_dir }}/Dockerfile .
    bats_flags=("-F" "tap")
    if [[ -n "${TERM:-}" ]]; then
      bats_flags=("--pretty")
    fi
    if [[ "${BATS_DEBUG:-}" == "true" ]]; then
      bats_flags+=("--trace" "--verbose-run")
    fi
    docker run --rm go-libyear-bats "${bats_flags[@]}" {{ test_dir }}/*.bats

# Run benchmark tests
test-benchmark:
    @{{ print_step }} "Running benchmark tests"
    go test -bench=. -benchmem ./...

# Produce test coverage report and inspect it in browser
test-coverage:
    @{{ print_step }} "Running test coverage report"
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out

# Run all checks
check: check-vet check-lint check-spell check-trailing check-markdown check-generate check-vulnerabilities

# Run 'go vet' on the whole project
check-vet:
    @{{ print_step }} "Running go vet"
    go vet ./...

# Run golangci-lint all-in-one linter with configuration defined inside .golangci.yml
check-lint:
    @{{ print_step }} "Running golangci-lint"
    golangci-lint run

# Check spelling, rules are defined in cspell.json
check-spell:
    @{{ print_step }} "Verifying spelling"
    cspell --no-progress '**/**'

# Check for trailing whitespaces in any of the projects' files
check-trailing:
    @{{ print_step }} "Looking for trailing whitespaces"
    {{ scripts_dir }}/check-trailing-whitespaces.bash

# Check markdown files for potential issues with markdownlint
check-markdown:
    @{{ print_step }} "Verifying Markdown files"
    markdownlint '**/*.md' --ignore node_modules

# Check for potential vulnerabilities across all Go dependencies
check-vulnerabilities:
    @{{ print_step }} "Running govulncheck"
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Verify if the auto generated code has been committed
check-generate:
    @{{ print_step }} "Checking if generated code matches the provided definitions"
    {{ scripts_dir }}/check-generate.bash

# Auto generate files
generate: generate-go

# Generate Golang code
generate-go:
    @{{ print_step }} "Generating Golang code"
    go generate ./...

# Format files
format: format-go format-just

# Format Go files
format-go:
    @{{ print_step }} "Formatting Go files"
    golangci-lint fmt

# Format justfile
format-just:
    @{{ print_step }} "Formatting justfile"
    just --fmt --unstable
