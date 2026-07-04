param(
  [string]$Version = "0.1.0",
  [string]$Registry = "https://registry.npmjs.org/"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

& (Join-Path $PSScriptRoot "build-npm.ps1") -Version $Version

Push-Location (Join-Path $root "npm\skills-win32-x64")
try {
  npm publish --access public --registry $Registry
  if ($LASTEXITCODE -ne 0) {
    throw "failed to publish @hxd/skills-win32-x64"
  }
} finally {
  Pop-Location
}

Push-Location (Join-Path $root "npm\skills")
try {
  npm publish --access public --registry $Registry
  if ($LASTEXITCODE -ne 0) {
    throw "failed to publish @hxd/skills"
  }
} finally {
  Pop-Location
}
