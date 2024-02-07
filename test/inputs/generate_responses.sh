#!/usr/bin/env bash

# Helper script to generate responses.json based on a list of repositories.

set -eo pipefail

test_go_mod="github.com/test/test"

repos=(
	github.com/pkg/errors
	golang.org/x/sync
	github.com/cpuguy83/go-md2man/v2
	github.com/xrash/smetrics
	github.com/BurntSushi/toml
	github.com/lestrrat-go/jwx
	github.com/lestrrat-go/jwx/v2
	github.com/go-playground/validator
	github.com/go-playground/validator/v10
)

json=""
for repo in "${repos[@]}"; do
	versions=$(go list -m -u -json -versions "$repo" | jq .Versions)
	if [[ $versions == "null" ]]; then
		versions=$(curl --silent "https://api.deps.dev/v3alpha/systems/go/packages/${repo//\//%2F}" |
			jq -c [.versions[].versionKey.version])
	fi

	escaped_repo=$(sed 's/[A-Z]/!\L&/g' <<<"$repo")

	info_endpoints=$(for version in $(jq -r .[] <<<"$versions"); do
		go list -m -json "$repo@$version" | jq '
		{([
				"'"$escaped_repo"'",
				"@v",
				 (.Version) + ".info"
		] | join("/")): {"Path": (.Path), "Version": (.Version), "Time": .Time}}'
	done | jq -s add)

	latest_version=$(jq -r .[-1] <<<"$versions")
	latest_time=$(jq -r 'to_entries | .[-1].value.Time' <<<"$info_endpoints")
	latest_endpoint=$(jq -n "{([
		\"$escaped_repo\",
		\"@latest\"
	] | join(\"/\")): {\"Path\": \"$repo\", \"Version\": \"$latest_version\", \"Time\": \"$latest_time\"}}")

	list_endpoint=$(jq "{([
		\"$escaped_repo\",
		\"@v\",
		\"list\"
	] | join(\"/\")): (. | join(\"\n\"))}" <<<"$versions")

	json="$json $info_endpoints $latest_endpoint $list_endpoint"
done

# We need .info and .mod for our test go.mod
json="$json {\"${test_go_mod}/@latest\": \"v1.0.0\"} {\"${test_go_mod}/@v/v1.0.0.mod\": \"./test/inputs/test-go.mod\"}"

jq -s 'reduce .[] as $obj ({}; . * $obj)' <<<"$json"
