#!/bin/sh
set -eu

ROLLING_REPO="${ROLLING_REPO:-https://github.com/ellanetworks/core.git}"

resolve_latest_release() {
    git ls-remote --tags --refs "${ROLLING_REPO}" 'v*' \
        | awk '{print $2}' \
        | sed 's#refs/tags/##' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
        | sort -V \
        | tail -1
}

if [ -z "${ROLLING_BASELINE_VERSION:-}" ]; then
    echo "==> Resolving latest release tag from ${ROLLING_REPO}"
    ROLLING_BASELINE_VERSION="$(resolve_latest_release)"

    if [ -z "${ROLLING_BASELINE_VERSION}" ]; then
        echo "error: could not resolve a latest release tag from ${ROLLING_REPO}." >&2
        echo "       Set ROLLING_BASELINE_VERSION explicitly to override." >&2
        exit 1
    fi
fi

ROLLING_BASELINE_IMAGE="ghcr.io/ellanetworks/ella-core:${ROLLING_BASELINE_VERSION}"

if ! docker image inspect ella-core:latest >/dev/null 2>&1; then
    echo "error: ella-core:latest not found in the local docker daemon." >&2
    echo "       Build it first with rockcraft / the standard image-build step." >&2
    exit 1
fi

echo "==> Baseline release: ${ROLLING_BASELINE_VERSION}"
echo "==> Pulling ${ROLLING_BASELINE_IMAGE}"
docker pull -q "${ROLLING_BASELINE_IMAGE}"

echo "==> Tagging as ella-core:rolling-baseline"
docker tag "${ROLLING_BASELINE_IMAGE}" ella-core:rolling-baseline

echo "==> Done."
echo "    ella-core:rolling-baseline  ($(docker image inspect -f '{{.Size}}' ella-core:rolling-baseline | numfmt --to=iec))"
echo "    ella-core:latest            ($(docker image inspect -f '{{.Size}}' ella-core:latest | numfmt --to=iec))"
