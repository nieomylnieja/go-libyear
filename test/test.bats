# setup_file is run once for the whole file.
setup_file() {
	load "test_helper/bats-support/load"
	load "test_helper/bats-assert/load"

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
}

# teardown_file is run once for the whole file after all tests finished.
teardown_file() {
  pkill test_server
}

# setup is run before each test.
setup() {
	load "test_helper/bats-support/load"
	load "test_helper/bats-assert/load"
}

@test "go_proxy: basic usage" {
	run golibyear "$INPUTS/go.mod"
	assert_success
	assert_output_equals basic_usage
}

@test "go_proxy: show indirect" {
	run golibyear --indirect "$INPUTS/go.mod"
	assert_success
	assert_output_equals show_indirect
}

@test "go_proxy: skip fresh" {
	run golibyear --skip-fresh "$INPUTS/go.mod"
	assert_success
	assert_output_equals skip_fresh
}

@test "go_proxy: skip fresh but show indirect" {
	run golibyear --skip-fresh --indirect "$INPUTS/go.mod"
	assert_success
	assert_output_equals skip_fresh_show_indirect
}

@test "go_proxy: pkg source" {
	run golibyear --pkg "github.com/test/test@v1.0.0"
	assert_success
	assert_output_equals basic_usage
}

@test "go_proxy: url source" {
	run golibyear --url "http://$SERVER_HOST:$SERVER_PORT/github.com/test/test/@v/v1.0.0.mod"
	assert_success
	assert_output_equals basic_usage
}

@test "go_proxy: show versions" {
	run golibyear --versions "$INPUTS/go.mod"
	assert_success
	assert_output_equals show_versions
}

@test "go_proxy: show releases" {
	run golibyear --releases "$INPUTS/go.mod"
	assert_success
	assert_output_equals show_releases
}

@test "go_proxy: show all details for all dependencies" {
	run golibyear --indirect --versions --releases "$INPUTS/go.mod"
	assert_success
	assert_output_equals all_details_for_all_dependencies
}

@test "go_proxy: csv output" {
	run golibyear --csv --versions --releases "$INPUTS/go.mod"
	assert_success
	assert_output_equals output.csv
}

@test "go_proxy: json output" {
	run golibyear --json --versions --releases "$INPUTS/go.mod"
	assert_success
	assert_output_equals output.json
}

@test "go_proxy: cache with XDG_CACHE_HOME" {
	export XDG_CACHE_HOME="$BATS_TEST_TMPDIR"
	run golibyear --cache "$INPUTS/go.mod"
	assert_success
	assert_cache_contents "$BATS_TEST_TMPDIR/golibyear/modules"
}

@test "go_proxy: cache with custom file path" {
	CACHE_FILE_PATH="$BATS_TEST_TMPDIR/custom-modules"
	run golibyear --cache --cache-file-path="$CACHE_FILE_PATH" "$INPUTS/go.mod"
	assert_success
	assert_cache_contents "$CACHE_FILE_PATH"
}

assert_cache_contents() {
	run cat "$1"
	assert_success
	assert_output --partial '{"path":"golang.org/x/sync","version":"0.5.0","time":"2023-10-11T14:04:17Z"}'
	assert_output --partial '{"path":"github.com/pkg/errors","version":"0.8.0","time":"2016-09-29T01:48:01Z"}'
	assert_output --partial '{"path":"github.com/BurntSushi/toml","version":"0.4.1","time":"2021-08-05T08:14:45Z"}'
	assert_output --partial '{"path":"github.com/pkg/errors","version":"0.9.1","time":"2020-01-14T19:47:44Z"}'
	assert_output --partial '{"path":"github.com/BurntSushi/toml","version":"1.3.2","time":"2023-06-08T06:14:45Z"}'
}

@test "error: non existent path" {
	run golibyear ./fake-path
	assert_failure
	assert_output "Error: open ./fake-path: no such file or directory"
}

@test "error: no path" {
	run golibyear
	assert_failure
	assert_output "Error: invalid number of arguments provided, expected a single argument, path to go.mod"
}

@test "error: stdin with forbidden args" {
	for arg in ./some/path --pkg --url; do
		run golibyear "$arg" - <<<"module github.com/nieomylnieja/golibyear"
		assert_failure
		assert_output "Error: when reading go.mod from stdin no arguments or output related flags should be provided"
	done
}

@test "error: cache file path without cache" {
	run golibyear --cache-file-path ./some/path
	assert_failure
	assert_output "Error: --cache-file-path flag can only be used in conjunction with --cache"
}

@test "error: timeout" {
	for alias in --timeout -t; do
		run golibyear --timeout 1ns "$INPUTS/go.mod"
		assert_failure
		assert_output --partial "context deadline exceeded"
	done
}

@test "error: invalid timeout" {
	for alias in --timeout -t; do
		run golibyear --timeout 1y "$INPUTS/go.mod"
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
	  run golibyear "${flagsArray[@]}" ./some/path
	  assert_failure
	  assert_output -e "use either --.* or --.* flag, but not both"
	done
}

@test "help flag" {
	for alias in --help -h; do
		run golibyear "$alias" >&1
		assert_output --partial "USAGE:"
	done
}

@test "version flag" {
	for alias in --version -v; do
		run golibyear "$alias"
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
