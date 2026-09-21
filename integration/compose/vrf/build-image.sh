#!/bin/sh
set -eu
cd "$(dirname "$0")"
docker build -t ella-core-vrf:latest -f Dockerfile .
