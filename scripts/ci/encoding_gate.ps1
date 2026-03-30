Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
Set-Location $repoRoot

function Invoke-GitLines([string]$argsLine) {
  $output = cmd /c "git $argsLine 2>nul"
  if ($null -eq $output) { return @() }
  if ($output -is [string]) { return @($output) }
  return @($output)
}

function Get-ChangedFiles {
  $files = New-Object System.Collections.Generic.HashSet[string] ([System.StringComparer]::OrdinalIgnoreCase)

  $diffFiles = @()
  $diffFiles += @(Invoke-GitLines "diff --name-only")
  $diffFiles += @(Invoke-GitLines "diff --cached --name-only")
  $diffFiles += @(Invoke-GitLines "ls-files --others --exclude-standard")

  foreach ($f in $diffFiles) {
    if ([string]::IsNullOrWhiteSpace($f)) { continue }
    $null = $files.Add($f.Trim())
  }

  if ($files.Count -eq 0) {
    $lastCommitFiles = @(Invoke-GitLines "show --pretty= --name-only HEAD")
    foreach ($f in $lastCommitFiles) {
      if ([string]::IsNullOrWhiteSpace($f)) { continue }
      $null = $files.Add($f.Trim())
    }
  }

  return @($files)
}

function IsTextLikeFile([string]$path) {
  $name = [System.IO.Path]::GetFileName($path)
  $ext = [System.IO.Path]::GetExtension($path).ToLowerInvariant()
  $textExt = @(
    ".go", ".js", ".ts", ".jsx", ".tsx",
    ".json", ".yaml", ".yml",
    ".md", ".txt", ".html", ".css", ".scss",
    ".sql", ".ps1", ".sh", ".bat", ".cmd",
    ".xml", ".toml", ".ini", ".conf", ".env"
  )

  if ($name -in @("Dockerfile", "Makefile")) { return $true }
  if ($textExt -contains $ext) { return $true }
  return $false
}

function IsBrowserStorageRelevantFile([string]$path) {
  $ext = [System.IO.Path]::GetExtension($path).ToLowerInvariant()
  $name = [System.IO.Path]::GetFileName($path)
  if ($ext -in @(".js", ".ts", ".jsx", ".tsx", ".html")) { return $true }
  if ($name -like "*.tmpl" -or $name -like "*.template") { return $true }
  return $false
}

function Get-DiffWithAdds {
  $diff = @(Invoke-GitLines "diff --no-color --unified=0")
  $diff += @(Invoke-GitLines "diff --cached --no-color --unified=0")
  return $diff
}

$changed = Get-ChangedFiles
Write-Host "Changed/new files considered by gate: $($changed.Count)" -ForegroundColor Cyan

# Gate 1: localStorage/sessionStorage additions in diff.
$storageHits = New-Object System.Collections.Generic.List[string]
foreach ($f in $changed) {
  if (-not (IsBrowserStorageRelevantFile $f)) { continue }
  $fileDiffLines = @(Invoke-GitLines "diff --no-color --unified=0 -- `"$f`"")
  $fileDiffLines += @(Invoke-GitLines "diff --cached --no-color --unified=0 -- `"$f`"")
  foreach ($line in $fileDiffLines) {
    if (-not $line.StartsWith("+")) { continue }
    if ($line.StartsWith("+++")) { continue }
    if ($line -match "localStorage|sessionStorage") {
      $storageHits.Add("$f :: $line")
    }
  }
}

foreach ($f in $changed) {
  if (-not (Test-Path -LiteralPath $f)) { continue }
  if (-not (IsBrowserStorageRelevantFile $f)) { continue }
  if ((Invoke-GitLines "ls-files --others --exclude-standard -- `"$f`"") -contains $f) {
    $text = Get-Content -LiteralPath $f -Raw -ErrorAction SilentlyContinue
    if ($null -ne $text -and ($text -match "localStorage|sessionStorage")) {
      $storageHits.Add("UNTRACKED:$f contains localStorage/sessionStorage")
    }
  }
}

if ($storageHits.Count -gt 0) {
  if ([string]::IsNullOrWhiteSpace($env:ALLOW_BROWSER_STORAGE_DIFF_JUSTIFICATION)) {
    Write-Host ""
    Write-Host "BLOCKED: localStorage/sessionStorage additions detected." -ForegroundColor Red
    Write-Host "Set ALLOW_BROWSER_STORAGE_DIFF_JUSTIFICATION with explicit rationale to proceed." -ForegroundColor Yellow
    Write-Host ""
    $storageHits | Select-Object -First 30 | ForEach-Object { Write-Host $_ }
    throw "Anti-browser-storage gate failed."
  } else {
    Write-Host "WARNING: browser storage changes accepted with justification:" -ForegroundColor Yellow
    Write-Host $env:ALLOW_BROWSER_STORAGE_DIFF_JUSTIFICATION
  }
}

# Gate 2: mojibake replacement character and UTF-8 validation.
$mojibakeHits = New-Object System.Collections.Generic.List[string]
$utf8Errors = New-Object System.Collections.Generic.List[string]
$strictUtf8 = New-Object System.Text.UTF8Encoding($false, $true)

foreach ($f in $changed) {
  if (-not (Test-Path -LiteralPath $f)) { continue }
  if (-not (IsTextLikeFile $f)) { continue }
  $item = Get-Item -LiteralPath $f
  if ($item.Length -gt 10MB) { continue }

  try {
    $bytes = [System.IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $f))
    $null = $strictUtf8.GetString($bytes)
  } catch {
    $utf8Errors.Add($f)
    continue
  }

  $content = Get-Content -LiteralPath $f -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
  if ($null -ne $content -and $content.Contains([char]0xFFFD)) {
    $mojibakeHits.Add($f)
  }
}

if ($utf8Errors.Count -gt 0 -or $mojibakeHits.Count -gt 0) {
  Write-Host ""
  Write-Host "BLOCKED: UTF-8/mojibake gate failed." -ForegroundColor Red
  if ($utf8Errors.Count -gt 0) {
    Write-Host "Invalid UTF-8 files:"
    $utf8Errors | ForEach-Object { Write-Host " - $_" }
  }
  if ($mojibakeHits.Count -gt 0) {
    Write-Host "Files containing replacement character U+FFFD:"
    $mojibakeHits | ForEach-Object { Write-Host " - $_" }
  }
  throw "Encoding gate failed."
}

Write-Host ""
Write-Host "Encoding and storage diff gate passed." -ForegroundColor Green
