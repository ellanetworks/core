#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
ui="$(cd "$here/../.." && pwd)"
export ELLA_E2E_OIDC_DIR="${ELLA_E2E_OIDC_DIR:-$ui/e2e/.build/oidc}"
mkdir -p "$ELLA_E2E_OIDC_DIR"
ELLA_E2E_OIDC_DIR="$(cd "$ELLA_E2E_OIDC_DIR" && pwd)"
export ELLA_E2E_CORE_BINARY="$ELLA_E2E_OIDC_DIR/core"
project="${ELLA_E2E_OIDC_PROJECT:-ella-core-oidc-e2e}"
compose=(docker compose -p "$project" -f "$here/compose.yaml" -f "$here/compose.local.yaml")

(cd "$ui" && npm run build)
(cd "$ui/.." && go build -o "$ELLA_E2E_CORE_BINARY" ./cmd/core)

down() {
  status=$?
  if [ "$status" -ne 0 ]; then
    "${compose[@]}" logs > "$ELLA_E2E_OIDC_DIR/compose.log" 2>&1 || true
    echo "Logs: $ELLA_E2E_OIDC_DIR/compose.log"
  fi
  "${compose[@]}" down -v > /dev/null 2>&1 || true
  exit "$status"
}
trap down EXIT

"$here/up.sh" -f "$here/compose.local.yaml"
(cd "$ui" && npx playwright test -c e2e/oidc/playwright.config.ts "$@")
