#!/usr/bin/env bash
set -euo pipefail

dir="${1:-$(dirname "$0")/../.build/oidc}"
mkdir -p "$dir"
cd "$dir"

openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 2 \
  -subj "/CN=Ella Core E2E CA" -keyout ca-key.pem -out ca.pem 2>/dev/null

issue() {
  local name="$1" san="$2"
  openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes \
    -subj "/CN=$name" -keyout "$name-key.pem" -out "$name.csr" 2>/dev/null
  openssl x509 -req -in "$name.csr" -CA ca.pem -CAkey ca-key.pem -CAcreateserial -days 2 \
    -extfile <(printf 'subjectAltName=%s\nextendedKeyUsage=serverAuth\n' "$san") \
    -out "$name.pem" 2>/dev/null
  rm "$name.csr"
}

issue keycloak "DNS:keycloak,DNS:localhost"
issue core "DNS:localhost,IP:127.0.0.1"

chmod 0644 ./*.pem
