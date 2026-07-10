#!/usr/bin/env bash

set -euo pipefail

tmp_dir=$(mktemp -d)
repo_dir="$tmp_dir/repo"

trap 'rm -rf "$tmp_dir"' EXIT

copy_worktree() {
  mkdir -p "$repo_dir"

  while IFS= read -r -d '' file; do
    if [[ ! -e "$file" ]]; then
      continue
    fi

    mkdir -p "$repo_dir/$(dirname "$file")"
    cp -Pp -- "$file" "$repo_dir/$file"
  done < <(git ls-files --cached --others --exclude-standard -z)
}

main() {
  copy_worktree
  git -C "$repo_dir" init --quiet
  git -C "$repo_dir" add --all

  just --justfile "$repo_dir/justfile" --working-directory "$repo_dir" generate

  changed=$(git -C "$repo_dir" diff --name-status -- '*.go' '*.yaml' '*.json')
  untracked=$(git -C "$repo_dir" ls-files --others --exclude-standard -- '*.go' '*.yaml' '*.json')
  if [[ -n "$untracked" ]]; then
    changed+=$'\n'
    changed+="$untracked"
  fi
  if [[ -n "$changed" ]]; then
    printf >&2 "There are generated changes that are not committed:\n%s\n" "$changed"
    exit 1
  fi

  echo "Looks good!"
}

main "$@"
