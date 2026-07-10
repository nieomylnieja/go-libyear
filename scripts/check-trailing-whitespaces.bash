#!/usr/bin/env bash

set -euo pipefail

file_extensions_to_ignore=(".ico" ".png" ".desc" ".gif")
found=0

while IFS= read -r file; do
  for ext in "${file_extensions_to_ignore[@]}"; do
    if [[ "$file" == *"$ext" ]]; then
      continue 2
    fi
  done
  if [[ ! -f "$file" ]]; then
    continue
  fi
  lines=$(rg --line-number --no-heading --no-messages ' +$' -- "$file" || true)
  if [[ -n "$lines" ]]; then
    if ((found != 1)); then
      echo "Trailing whitespaces found!" >&2
    fi
    while IFS=: read -r _ line _; do
      echo "$file:$line" >&2
    done <<< "$lines"
    found=1
  fi
done < <(git ls-tree -r HEAD --name-only)

if ((found == 1)); then
  exit 1
fi
