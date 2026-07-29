#!/usr/bin/env bash
# SPDX-FileCopyrightText: Ella Networks Inc.
# SPDX-License-Identifier: BUSL-1.1
#
# Runs every fuzz target in the module, each for the same budget.
#
#   ./fuzz.sh            # 5 minutes per target
#   ./fuzz.sh 2h         # 2 hours per target
#   ./fuzz.sh 30m FuzzParseMessage
#
# A failing input is written to the package's testdata/fuzz/<target>/ and is
# picked up by `go test` from then on, so commit it with the fix.
set -u

cd "$(dirname "$0")" || exit 1

budget=${1:-5m}
filter=${2:-}
status=0

# A target name is unique only within its package: both generations define
# FuzzParseMessage, FuzzParseSecurityProtectedMessage and FuzzIECodecs.
pairs=$(for file in $(find . -name '*_test.go' | sort); do
	for target in $(sed -n 's/^func \(Fuzz[A-Za-z0-9_]*\)(.*/\1/p' "$file"); do
		echo "$(dirname "$file") $target"
	done
done | sort -u)

while read -r pkg target; do
	[ -n "$target" ] || continue

	if [ -n "$filter" ] && [ "$target" != "$filter" ]; then
		continue
	fi

	printf '\n=== %s %s for %s\n' "$pkg" "$target" "$budget"

	if ! go test "$pkg" -run '^$' -fuzz "^$target\$" -fuzztime "$budget"; then
		status=1
	fi
done <<EOF
$pairs
EOF

exit $status
