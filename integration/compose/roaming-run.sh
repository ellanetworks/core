#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 {4g|5g} {images|test [go test -run regex]|down|logs [service]}" >&2
  exit 1
}

case "${1:-}" in
  4g) TEST=TestRoaming4GOpen5GS ;;
  5g) TEST=TestRoaming5GOpen5GS ;;
  *) usage ;;
esac

RAT="$1"
shift

cd "$(dirname "$0")/roaming$RAT"
ROOT="$(cd ../../.. && pwd)"

ELLA_IMAGE="${ELLA_IMAGE:-ella-core:roaming$RAT}"
TESTER_IMAGE="${TESTER_IMAGE:-ella-core-tester:roaming$RAT}"
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
      go test -tags open5gs -count=1 -v -timeout 45m -run "${2:-$TEST}" .
    ;;
  down)
    docker compose -f docker-compose.yaml down -v --remove-orphans
    ;;
  logs)
    docker compose -f docker-compose.yaml logs -f "${@:2}"
    ;;
  *)
    usage
    ;;
esac
