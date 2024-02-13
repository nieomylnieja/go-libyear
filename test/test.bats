# setup_file is run once for the whole file.
setup_file() {
  # Current date for output testing.
  MAIN_DATE="$(date +'%Y-%m-%d')"
  export MAIN_DATE

	# Common directories.
	export INPUTS="$BATS_TEST_DIRNAME/inputs"
	export OUTPUTS="$BATS_TEST_DIRNAME/outputs"

  # Run test server.
  SERVER_PORT="8091"
  SERVER_HOST="127.0.0.1"
	test_server -port "$SERVER_PORT" -path "$INPUTS/responses.json" &
	# Wait for server to start.
	for (( i=0; i<50; i++ )); do
	  nc -z -w 2 "$SERVER_HOST" "$SERVER_PORT" && break
	  sleep 0.1
	done
	export SERVER_PORT
	export SERVER_HOST
	export GOPROXY="http://$SERVER_HOST:$SERVER_PORT"
  export TEST_GO_MOD="$INPUTS/test-go.mod"
  export TEST_PRIVATE_GO_MOD="$INPUTS/private-go.mod"
}

# teardown_file is run once for the whole file after all tests finished.
teardown_file() {
  pkill test_server
}

# setup is run before each test.
setup() {
	load_lib "bats-support"
	load_lib "bats-assert"
}

@test "go_proxy: basic usage" {
	run go-libyear "$TEST_GO_MOD"
	assert_success
	assert_output_equals basic_usage
}

@test "go_proxy: show indirect" {
	run go-libyear --indirect "$TEST_GO_MOD"
	assert_success
	assert_output_equals show_indirect
}

@test "go_proxy: skip fresh" {
	run go-libyear --skip-fresh "$TEST_GO_MOD"
	assert_success
	assert_output_equals skip_fresh
}

@test "go_proxy: skip fresh but show indirect" {
	run go-libyear --skip-fresh --indirect "$TEST_GO_MOD"
	assert_success
	assert_output_equals skip_fresh_show_indirect
}

@test "go_proxy: pkg source" {
	run go-libyear --pkg "github.com/test/test@v1.0.0"
	assert_success
	assert_output_equals basic_usage
}

@test "go_proxy: url source" {
	run go-libyear --url "http://$SERVER_HOST:$SERVER_PORT/github.com/test/test/@v/v1.0.0.mod"
	assert_success
	assert_output_equals basic_usage
}

@test "go_proxy: show versions" {
	run go-libyear --versions "$TEST_GO_MOD"
	assert_success
	assert_output_equals show_versions
}

@test "go_proxy: show releases" {
	run go-libyear --releases "$TEST_GO_MOD"
	assert_success
	assert_output_equals show_releases
}

@test "go_proxy: show all details for all dependencies" {
	run go-libyear --indirect --versions --releases "$TEST_GO_MOD"
	assert_success
	assert_output_equals all_details_for_all_dependencies
}

@test "go_proxy: csv output, minimal" {
	run go-libyear --csv "$TEST_GO_MOD"
	assert_success
	assert_output_equals output-minimal.csv
}

@test "go_proxy: csv output, full" {
	run go-libyear --csv --versions --releases --indirect "$TEST_GO_MOD"
	assert_success
	assert_output_equals output-full.csv
}

@test "go_proxy: json output, minimal" {
	run go-libyear --json "$TEST_GO_MOD"
	assert_success
	assert_output_equals output-minimal.json
}

@test "go_proxy: json output, full" {
	run go-libyear --json --versions --releases --indirect "$TEST_GO_MOD"
	assert_success
	assert_output_equals output-full.json
}

@test "go_proxy: find latest major, full" {
  bats_require_minimum_version 1.5.0
	run --separate-stderr go-libyear -M --versions --releases --indirect "$TEST_GO_MOD"
	assert_success
	assert_output_equals all_with_latest_major_versions
}

@test "go_proxy: find latest major, full, no compensate" {
	run go-libyear -M --no-libyear-compensation --versions --releases --indirect "$TEST_GO_MOD"
	assert_success
	assert_output_equals all_with_latest_major_versions_no_compensate
}

@test "go_proxy: go mod with goprivate" {
  export GOPRIVATE=github.com/nieomylnieja/*
	run go-libyear --versions --releases --indirect "$TEST_PRIVATE_GO_MOD"
	assert_success
	assert_output_equals all_with_private_module
}

@test "go_proxy: goprivate pkg" {
  export GOPRIVATE=github.com/nieomylnieja/*
	run go-libyear --versions --releases --indirect --pkg github.com/nieomylnieja/private-go-module-test
	assert_success
	assert_output_equals all_for_private_pkg
}

@test "go_proxy: goprivate pkg with @latest" {
  export GOPRIVATE=github.com/nieomylnieja/*
	run go-libyear --versions --releases --indirect --pkg github.com/nieomylnieja/private-go-module-test@latest
	assert_success
	assert_output_equals all_for_private_pkg_latest
}

@test "go_proxy: goprivate pkg with v0.3.0" {
  export GOPRIVATE=github.com/nieomylnieja/*
	run go-libyear --versions --releases --indirect --pkg github.com/nieomylnieja/private-go-module-test@v0.3.0
	assert_success
	assert_output_equals all_for_private_pkg_v0.3.0
}

@test "go_proxy: cache with XDG_CACHE_HOME" {
	export XDG_CACHE_HOME="$BATS_TEST_TMPDIR"
	run go-libyear --cache "$TEST_GO_MOD"
	assert_success
	assert_cache_contents "$BATS_TEST_TMPDIR/go-libyear/modules"
}

@test "go_proxy: cache with custom file path" {
	CACHE_FILE_PATH="$BATS_TEST_TMPDIR/custom-modules"
	run go-libyear --cache --cache-file-path="$CACHE_FILE_PATH" "$TEST_GO_MOD"
	assert_success
	assert_cache_contents "$CACHE_FILE_PATH"
}

@test "go_proxy: vcs cache with XDG_CACHE_HOME" {
	export XDG_CACHE_HOME="$BATS_TEST_TMPDIR"
  export GOPRIVATE=github.com/nieomylnieja/*
	run go-libyear "$TEST_PRIVATE_GO_MOD"
	assert_success
	assert_vcs_cache_contents "$BATS_TEST_TMPDIR/go-libyear/vcs"
}

@test "go_proxy: vcs cache with custom file path" {
	CACHE_FILE_PATH="$BATS_TEST_TMPDIR/vcs"
  export GOPRIVATE=github.com/nieomylnieja/*
	run go-libyear --vcs-cache-dir="$CACHE_FILE_PATH" "$TEST_PRIVATE_GO_MOD"
	assert_success
	assert_vcs_cache_contents "$CACHE_FILE_PATH"
}

assert_cache_contents() {
	run sort "$1"
	assert_success
	assert_output - <<EOF
{"path":"github.com/BurntSushi/toml","version":"0.4.1","time":"2021-08-05T08:14:45Z"}
{"path":"github.com/BurntSushi/toml","version":"1.3.2","time":"2023-06-08T06:14:45Z"}
{"path":"github.com/go-playground/validator","version":"8.18.2+incompatible","time":"2017-07-30T05:02:35Z"}
{"path":"github.com/go-playground/validator","version":"9.31.0+incompatible","time":"2019-12-25T05:24:06Z"}
{"path":"github.com/lestrrat-go/jwx","version":"1.2.28","time":"2024-01-09T01:52:35Z"}
{"path":"github.com/pkg/errors","version":"0.8.0","time":"2016-09-29T01:48:01Z"}
{"path":"github.com/pkg/errors","version":"0.9.1","time":"2020-01-14T19:47:44Z"}
{"path":"golang.org/x/sync","version":"0.5.0","time":"2023-10-11T14:04:17Z"}
{"path":"golang.org/x/sync","version":"0.6.0","time":"2023-12-07T16:58:19Z"}
EOF
}

assert_vcs_cache_contents() {
	run find "$1" -not -path '*/.git*'
	assert_success
	assert_output - <<EOF
$1
$1/github.com
$1/github.com/nieomylnieja
$1/github.com/nieomylnieja/private-go-module-test
$1/github.com/nieomylnieja/private-go-module-test/test.go
$1/github.com/nieomylnieja/private-go-module-test/README.md
$1/github.com/nieomylnieja/private-go-module-test/go.sum
$1/github.com/nieomylnieja/private-go-module-test/go.mod
EOF
}

@test "error: non existent path" {
	run go-libyear ./fake-path
	assert_failure
	assert_output "Error: open ./fake-path: no such file or directory"
}

@test "error: no path" {
	run go-libyear
	assert_failure
	assert_output "Error: invalid number of arguments provided, expected a single argument, path to go.mod"
}

@test "error: stdin with forbidden args" {
	for arg in ./some/path --pkg --url; do
		run go-libyear "$arg" - <<<"module github.com/nieomylnieja/go-libyear"
		assert_failure
		assert_output "Error: when reading go.mod from stdin no arguments or output related flags should be provided"
	done
}

@test "error: cache file path without cache" {
	run go-libyear --cache-file-path ./some/path
	assert_failure
	assert_output "Error: --cache-file-path flag can only be used in conjunction with --cache"
}

@test "error: compensate flag without major version flag" {
	run go-libyear --no-libyear-compensation ./some/path
	assert_failure
	assert_output "Error: --no-libyear-compensation flag can only be used in conjunction with --find-latest-major"
}

@test "error: timeout" {
	for alias in --timeout -t; do
		run go-libyear --timeout 1ns "$TEST_GO_MOD"
		assert_failure
		assert_output --partial "1ns timeout exceeded, consider increasing the timeout value via --timeout flag"
	done
}

@test "error: invalid timeout" {
	for alias in --timeout -t; do
		run go-libyear --timeout 1y "$TEST_GO_MOD"
		assert_failure
		assert_output --partial "parse error"
	done
}

@test "error: conflicting flags" {
	allFlags=(
	    "--json --csv"
	    "--url --pkg"
	    "--go-list --pkg"
	)
	for flags in "${allFlags[@]}"; do
	  IFS=' ' read -r -a flagsArray <<< "$flags"
	  run go-libyear "${flagsArray[@]}" ./some/path
	  assert_failure
	  assert_output -e "use either --.* or --.* flag, but not both"
	done
}

@test "help flag" {
	for alias in --help -h; do
		run go-libyear "$alias" >&1
		assert_output --partial "USAGE:"
	done
}

@test "version flag" {
	for alias in --version -v; do
		run go-libyear "$alias"
		assert_output - <<EOF
Version: 2.0.0
GitTag: v2.0.0
BuildDate: 2023-10-23T08:03:03Z
EOF
	done
}

assert_output_equals() {
	assert_output "$(MAIN_DATE="$MAIN_DATE" envsubst <"$OUTPUTS/$1")"
}

load_lib() {
  local name="$1"
  load "/usr/lib/bats/${name}/load.bash"
}
