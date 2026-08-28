#!/usr/bin/env bash
set -euo pipefail
root=$(pwd)
out=""
for mod in $(find . -name go.mod -not -path '*/node_modules/*' | xargs -n1 dirname | sed 's|^\./||' | sort); do
  cd "$root/$mod"
  prefix=$(go list -m)
  for imp in $(go list ./... 2>/dev/null); do
    pkg=".${imp#"$prefix"}"
    for tgt in $(go test "$pkg" -list '^Fuzz' 2>/dev/null | grep '^Fuzz' || true); do
      out="${out}{\"dir\":\"$mod\",\"pkg\":\"$pkg\",\"target\":\"$tgt\"},"
    done
  done
done
cd "$root"

if [ -z "$out" ]; then
  echo "no fuzz targets found; the discovery script is broken" >&2
  exit 1
fi

echo "[${out%,}]"
