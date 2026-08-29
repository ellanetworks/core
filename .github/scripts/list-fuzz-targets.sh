#!/usr/bin/env bash
set -euo pipefail
set -f
root=$(pwd)
out=""
count=0

modules=$(find . -name node_modules -prune -o -name go.mod -print | xargs -n1 dirname | sed 's|^\./||' | sort)

for mod in $modules; do
  cd "$root/$mod"

  prefix=$(go list -m)

  if ! imports=$(go list ./... 2>&1); then
    echo "go list ./... failed in module $mod; discovery would silently skip its fuzz targets" >&2
    echo "$imports" >&2
    exit 1
  fi

  for imp in $imports; do
    pkg=".${imp#"$prefix"}"

    if ! listing=$(go test "$pkg" -list '^Fuzz' 2>&1); then
      echo "go test -list failed for $mod/$pkg; discovery would silently skip its fuzz targets" >&2
      echo "$listing" >&2
      exit 1
    fi

    for tgt in $(printf '%s\n' "$listing" | grep '^Fuzz' || true); do
      out="${out}{\"dir\":\"$mod\",\"pkg\":\"$pkg\",\"target\":\"$tgt\"},"
      count=$((count + 1))
    done
  done
done

cd "$root"

if [ "$count" -eq 0 ]; then
  echo "no fuzz targets found; the discovery script is broken" >&2
  exit 1
fi

if [ "$count" -gt 256 ]; then
  echo "$count fuzz targets exceeds the 256-job GitHub Actions matrix limit" >&2
  exit 1
fi

echo "[${out%,}]"
