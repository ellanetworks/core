#!/bin/sh
# Prepare images used by TestIntegrationHARollingUpgrade.
#
#   ella-core:rolling-baseline — the previous release image, pulled from
#                                ghcr.io. Bump ROLLING_BASELINE_VERSION
#                                with every release.
#   ella-core:latest           — the current build (already produced by
#                                the standard image-build step).
#
# The test rolls the cluster from rolling-baseline to latest and surfaces
# any rolling-upgrade compatibility break (renamed or removed Raft op,
# schema migration that an old node cannot tolerate, payload-shape drift)
# as a real test failure rather than a stale committed manifest.
set -eu

# Pinned to the previous release tag. Bump this in the same PR that cuts
# a new release.
ROLLING_BASELINE_VERSION="${ROLLING_BASELINE_VERSION:-v1.10.1}"
ROLLING_BASELINE_IMAGE="ghcr.io/ellanetworks/ella-core:${ROLLING_BASELINE_VERSION}"

if ! docker image inspect ella-core:latest >/dev/null 2>&1; then
    echo "error: ella-core:latest not found in the local docker daemon." >&2
    echo "       Build it first with rockcraft / the standard image-build step." >&2
    exit 1
fi

echo "==> Pulling ${ROLLING_BASELINE_IMAGE}"
docker pull -q "${ROLLING_BASELINE_IMAGE}"

echo "==> Tagging as ella-core:rolling-baseline"
docker tag "${ROLLING_BASELINE_IMAGE}" ella-core:rolling-baseline

echo "==> Done."
echo "    ella-core:rolling-baseline  ($(docker image inspect -f '{{.Size}}' ella-core:rolling-baseline | numfmt --to=iec))"
echo "    ella-core:latest            ($(docker image inspect -f '{{.Size}}' ella-core:latest | numfmt --to=iec))"
