param(
  [string]$Version = "0.1.0"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$workspaceRoot = Split-Path $root -Parent
$goCandidates = @(
  (Join-Path $root ".tools\go1.25.11\go\bin\go.exe"),
  (Join-Path $workspaceRoot ".tools\go1.25.11\go\bin\go.exe")
)
$go = $goCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1

if (-not $go) {
  $goCmd = Get-Command go -ErrorAction SilentlyContinue
  if (-not $goCmd) {
    throw "Go is not available. Expected portable Go under .tools or go in PATH."
  }
  $go = $goCmd.Source
}

function Set-JsonVersion {
  param(
    [string]$Path,
    [string]$Version
  )

  $json = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json
  $json.version = $Version
  if ($json.optionalDependencies -and $json.optionalDependencies.PSObject.Properties.Name -contains "@xiaodonghu/skills-win32-x64") {
    $json.optionalDependencies."@xiaodonghu/skills-win32-x64" = $Version
  }
  $json | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $Path -Encoding UTF8
}

Set-JsonVersion -Path (Join-Path $root "package.json") -Version $Version
Set-JsonVersion -Path (Join-Path $root "npm\skills\package.json") -Version $Version
Set-JsonVersion -Path (Join-Path $root "npm\skills-win32-x64\package.json") -Version $Version

$binDir = Join-Path $root "npm\skills-win32-x64\bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

Push-Location $root
try {
  & $go build -trimpath -ldflags "-s -w" -o (Join-Path $binDir "skills.exe") ./cmd/skills
  if ($LASTEXITCODE -ne 0) {
    throw "go build failed"
  }
} finally {
  Pop-Location
}

Write-Host "Built npm package binary: $(Join-Path $binDir "skills.exe")"

