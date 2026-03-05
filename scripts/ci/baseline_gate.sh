#!/usr/bin/env bash
set -euo pipefail

if [ -d ./local_checks ]; then
  echo "[baseline] local encoding checks"
  go test ./local_checks -count=1
else
  echo "[baseline] local_checks not present, skip"
fi

echo "[baseline] config validation tests"
go test ./config -count=1

echo "[baseline] monitoring core tests"
go test ./core/monitoring -count=1

echo "[baseline] OK"