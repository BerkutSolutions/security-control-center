#!/usr/bin/env bash
set -euo pipefail

echo "[upgrade] goose migration sequence"
go test ./core/store -run TestGooseMigrationsSequenceNoGaps -count=1

echo "[upgrade] sqlite latest-1 smoke"
go test ./core/store -run TestSQLiteLatestMinusOneUpgradeSmoke -count=1

echo "[upgrade] OK"