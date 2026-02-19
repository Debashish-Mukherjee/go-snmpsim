#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RUN_V2=1
RUN_V3=0
RUN_SOAK=0
SOAK_DURATION="10m"

while [[ $# -gt 0 ]]; do
	case "$1" in
		--v2-only)
			RUN_V2=1
			RUN_V3=0
			shift
			;;
		--v3-only)
			RUN_V2=0
			RUN_V3=1
			shift
			;;
		--with-v3)
			RUN_V3=1
			shift
			;;
		--soak)
			RUN_SOAK=1
			shift
			;;
		--duration)
			SOAK_DURATION="${2:-10m}"
			shift 2
			;;
		*)
			echo "Unknown arg: $1"
			echo "Usage: $0 [--v2-only|--v3-only|--with-v3] [--soak] [--duration 10m]"
			exit 1
			;;
	esac
done

echo "[stress-2000] Running SNMPSIM stress suite (build tag: stress)..."

if [[ "$RUN_V2" -eq 1 ]]; then
	echo "[stress-2000] v2c 2000-device stress"
	go test -tags stress ./internal/engine -run TestStress2000CiscoIOSDevices$ -v -count=1 -timeout 20m
fi

if [[ "$RUN_V3" -eq 1 ]]; then
	echo "[stress-2000] v3(noAuthNoPriv) 2000-device stress"
	SNMPSIM_STRESS_V3=1 go test -tags stress ./internal/engine -run TestStress2000CiscoIOSDevicesV3NoAuthNoPriv$ -v -count=1 -timeout 20m
fi

if [[ "$RUN_SOAK" -eq 1 ]]; then
	echo "[stress-2000] soak mode duration=${SOAK_DURATION}"
	SNMPSIM_STRESS_SOAK=1 SNMPSIM_STRESS_SOAK_DURATION="$SOAK_DURATION" \
		go test -tags stress ./internal/engine -run TestStressSoak10Minutes$ -v -count=1 -timeout 30m
fi

echo "[stress-2000] PASS"
