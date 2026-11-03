#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(cd ../../.. && pwd)"

ELLA_IMAGE="${ELLA_IMAGE:-ella-core:roaming5g}"
TESTER_IMAGE="${TESTER_IMAGE:-ella-core-tester:roaming5g}"
ELLA_BASE="${ELLA_BASE:-ella-core:latest}"
TESTER_BASE="${TESTER_BASE:-ella-core-tester:latest}"

images() {
  local dir
  dir="$(mktemp -d)"
  trap 'rm -rf "$dir"' RETURN
  (cd "$ROOT" && go build -o "$dir/core" ./cmd/core && go build -o "$dir/core-tester" ./cmd/core-tester)
  printf 'FROM %s\nCOPY core /bin/core\n' "$ELLA_BASE" > "$dir/Dockerfile.core"
  printf 'FROM %s\nCOPY core-tester /usr/bin/core-tester\n' "$TESTER_BASE" > "$dir/Dockerfile.tester"
  docker build -q -f "$dir/Dockerfile.core" -t "$ELLA_IMAGE" "$dir"
  docker build -q -f "$dir/Dockerfile.tester" -t "$TESTER_IMAGE" "$dir"
}

case "${1:-}" in
  images)
    images
    ;;
  test)
    images
    cd "$ROOT/integration"
    ELLA_IMAGE="$ELLA_IMAGE" TESTER_IMAGE="$TESTER_IMAGE" \
      go test -tags open5gs -count=1 -v -timeout 45m -run "${2:-TestRoaming5GOpen5GS}" .
    ;;
  down)
    docker compose -f docker-compose.yaml down -v --remove-orphans
    ;;
  logs)
    docker compose -f docker-compose.yaml logs -f "${@:2}"
    ;;
  *)
    echo "usage: $0 {images|test [go test -run regex]|down|logs [service]}" >&2
    exit 1
    ;;
esac
