#!/bin/sh
set -eu

ROLLING_REPO="${ROLLING_REPO:-https://github.com/ellanetworks/core.git}"
ROLLING_IMAGE_REPO="${ROLLING_IMAGE_REPO:-ghcr.io/ellanetworks/ella-core}"

list_release_tags() {
    git ls-remote --tags --refs "${ROLLING_REPO}" 'v*' \
        | awk '{print $2}' \
        | sed 's#refs/tags/##' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
        | sort -V \
        | tail -2 \
        | sed '1!G;h;$!d'
}

resolve_latest_release() {
    list_release_tags | while IFS= read -r tag; do
        if [ -z "${tag}" ]; then
            continue
        fi
        if docker manifest inspect "${ROLLING_IMAGE_REPO}:${tag}" >/dev/null 2>&1; then
            printf '%s\n' "${tag}"
            return 0
        fi
        echo "==> Skipping ${tag}: no published image yet" >&2
    done | head -1
}

if [ -z "${ROLLING_BASELINE_VERSION:-}" ]; then
    echo "==> Resolving latest published release from ${ROLLING_REPO}"
    ROLLING_BASELINE_VERSION="$(resolve_latest_release)"

    if [ -z "${ROLLING_BASELINE_VERSION}" ]; then
        echo "error: neither of the two newest releases from ${ROLLING_REPO} has a" >&2
        echo "       published image in ${ROLLING_IMAGE_REPO}; the release pipeline is likely broken." >&2
        echo "       Set ROLLING_BASELINE_VERSION explicitly to override." >&2
        exit 1
    fi
fi

ROLLING_BASELINE_IMAGE="${ROLLING_IMAGE_REPO}:${ROLLING_BASELINE_VERSION}"

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
