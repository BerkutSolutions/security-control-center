Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

if (-not $env:GOCACHE -or [string]::IsNullOrWhiteSpace($env:GOCACHE)) {
  $env:GOCACHE = Join-Path $repoRoot ".gocache"
}

$commands = @(
  '.\scripts\ci\encoding_gate.ps1',
  'go test -count=1 ./...',
  'go test -count=1 ./core/store -run "TestSQLiteLatestMinusOneUpgrade"',
  'go test -count=1 ./api -run "TestSecurityNegative_"',
  'go test -count=1 ./api/handlers -run "TestAuditContract|TestAPISchemaContract"',
  'go test -count=1 ./tests -run "TestI18NRuntime|TestI18NKeyCoverage|TestI18N_NoArtifacts"',
  'go test -count=1 ./local_checks'
)

foreach ($cmd in $commands) {
  Write-Host ""
  Write-Host "==> $cmd" -ForegroundColor Cyan
  Invoke-Expression $cmd
  if ($LASTEXITCODE -ne 0) {
    throw "Command failed with exit code ${LASTEXITCODE}: $cmd"
  }
}

Write-Host ""
Write-Host "Enterprise test gate passed." -ForegroundColor Green
