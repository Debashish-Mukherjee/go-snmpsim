#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[stress-2000] Running SNMPSIM 2000-device stress suite..."
echo "[stress-2000] This runs internal/engine stress tests with build tag: stress"

go test -tags stress ./internal/engine -run TestStress2000CiscoIOSDevices -v -count=1 -timeout 20m

echo "[stress-2000] PASS"
