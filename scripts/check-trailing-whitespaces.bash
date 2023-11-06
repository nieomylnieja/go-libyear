#!/usr/bin/env bash

found=0
for file in $(git ls-tree -r HEAD --name-only); do
	if [ ! -f "$file" ]; then
		continue
	fi
  if grep -qE ' +$' "$file"; then
  	if ((found != 1)); then
    	echo "Trailing whitespaces found!" >&2
  	fi
  	grep -nE ' +$' "$file" | awk -F: '{print $1}' | while IFS= read -r line; do
  		echo "$file:$line" >&2
  	done
    found=1
  fi
done
if ((found == 1)); then
	exit 1
fi