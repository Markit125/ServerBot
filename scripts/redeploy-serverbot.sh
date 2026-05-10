#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

if [[ "${1:-}" == "--reload-units" ]]; then
    make reload-units
    shift
fi

if [[ "${1:-}" == "--restart-bot-api" ]]; then
    make restart-bot-api
    shift
fi

if [[ $# -gt 0 ]]; then
    echo "Usage: $0 [--reload-units] [--restart-bot-api]" >&2
    exit 1
fi

make deploy
