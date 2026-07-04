param(
  [string]$Version = "0.1.0",
  [string]$Registry = "https://registry.npmjs.org/",
  [string]$SecretsPath = "D:\ToolManage\.secrets\npm.env"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

function Import-NpmToken {
  param([string]$Path)

  if (-not [string]::IsNullOrWhiteSpace($env:NPM_TOKEN)) {
    return
  }

  if (-not (Test-Path $Path)) {
    throw "NPM_TOKEN is not set and secrets file was not found: $Path"
  }

  foreach ($line in Get-Content -LiteralPath $Path) {
    $trimmed = $line.Trim()
    if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
      continue
    }
    $parts = $trimmed.Split("=", 2)
    if ($parts.Length -eq 2 -and $parts[0].Trim() -eq "NPM_TOKEN") {
      $env:NPM_TOKEN = $parts[1].Trim()
      break
    }
  }

  if ([string]::IsNullOrWhiteSpace($env:NPM_TOKEN)) {
    throw "NPM_TOKEN is empty. Set it in environment or $Path."
  }
}

Import-NpmToken -Path $SecretsPath

$npmrc = Join-Path ([System.IO.Path]::GetTempPath()) ("skills-npm-" + [Guid]::NewGuid().ToString("N") + ".npmrc")
Set-Content -LiteralPath $npmrc -Value "//registry.npmjs.org/:_authToken=`${NPM_TOKEN}`n" -Encoding ASCII

try {
  & (Join-Path $PSScriptRoot "build-npm.ps1") -Version $Version

  Push-Location (Join-Path $root "npm\skills-win32-x64")
  try {
    npm publish --access public --registry $Registry --userconfig $npmrc
    if ($LASTEXITCODE -ne 0) {
      throw "failed to publish @xiaodonghu/skills-win32-x64"
    }
  } finally {
    Pop-Location
  }

  Push-Location (Join-Path $root "npm\skills")
  try {
    npm publish --access public --registry $Registry --userconfig $npmrc
    if ($LASTEXITCODE -ne 0) {
      throw "failed to publish @xiaodonghu/skills"
    }
  } finally {
    Pop-Location
  }
} finally {
  Remove-Item -LiteralPath $npmrc -Force -ErrorAction SilentlyContinue
}

