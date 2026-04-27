#!/bin/sh
# Build the two images used by TestIntegrationHARollingUpgrade:
#
#   ella-core:rolling-baseline — current source, no synthetic migrations.
#   ella-core:rolling-target   — same source built with
#                                -tags rolling_upgrade_test_synthetic,
#                                which appends three trivial no-op
#                                migrations to the registry.
#
# Both stamp their fresh binary onto ella-core:latest via Dockerfile.swap.
# Run from the repo root or from anywhere — the script resolves its own
# location and uses repo-root-relative paths for the Go source.
set -eu

DIR=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "${DIR}/../../.." && pwd)

# Sanity: ella-core:latest must exist (Dockerfile.swap FROMs it).
if ! docker image inspect ella-core:latest >/dev/null 2>&1; then
    echo "error: ella-core:latest not found in the local docker daemon." >&2
    echo "       Build it first with rockcraft / the standard image-build step." >&2
    exit 1
fi

cd "${REPO}"

echo "==> Building rolling-upgrade baseline binary"
go build -o "${DIR}/core" ./cmd/core

echo "==> Building ella-core:rolling-baseline image"
docker build -q -t ella-core:rolling-baseline -f "${DIR}/Dockerfile.swap" "${DIR}"

echo "==> Building rolling-upgrade target binary (tag: rolling_upgrade_test_synthetic)"
go build -tags rolling_upgrade_test_synthetic -o "${DIR}/core" ./cmd/core

echo "==> Building ella-core:rolling-target image"
docker build -q -t ella-core:rolling-target -f "${DIR}/Dockerfile.swap" "${DIR}"

# Clean the per-build binary; the image carries it from here.
rm -f "${DIR}/core"

echo "==> Done."
echo "    ella-core:rolling-baseline  ($(docker image inspect -f '{{.Size}}' ella-core:rolling-baseline | numfmt --to=iec))"
echo "    ella-core:rolling-target    ($(docker image inspect -f '{{.Size}}' ella-core:rolling-target | numfmt --to=iec))"
