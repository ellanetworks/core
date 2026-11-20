#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
export ELLA_E2E_OIDC_DIR="${ELLA_E2E_OIDC_DIR:-$here/../.build/oidc}"
mkdir -p "$ELLA_E2E_OIDC_DIR"
ELLA_E2E_OIDC_DIR="$(cd "$ELLA_E2E_OIDC_DIR" && pwd)"
project="${ELLA_E2E_OIDC_PROJECT:-ella-core-oidc-e2e}"
core_port="${ELLA_E2E_OIDC_PORT:-15444}"
keycloak_port="${KEYCLOAK_E2E_PORT:-18444}"
compose=(docker compose -p "$project" -f "$here/compose.yaml" "$@")

"$here/certs.sh" "$ELLA_E2E_OIDC_DIR"
"${compose[@]}" down -v
"${compose[@]}" up -d

ready() {
  for _ in $(seq 1 120); do
    if curl -sf -o /dev/null --cacert "$ELLA_E2E_OIDC_DIR/ca.pem" "$@"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

if ! ready --resolve "keycloak:$keycloak_port:127.0.0.1" \
  "https://keycloak:$keycloak_port/realms/ella/.well-known/openid-configuration"; then
  echo "Keycloak did not become ready in time"
  "${compose[@]}" logs
  exit 1
fi
echo "Keycloak is ready"

if ! ready "https://localhost:$core_port/api/v1/status"; then
  echo "Ella Core did not become ready in time"
  "${compose[@]}" logs
  exit 1
fi
echo "Ella Core is ready"
