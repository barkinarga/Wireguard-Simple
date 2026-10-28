$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
if (-not (Test-Path -LiteralPath '.\bin\wgsimple.exe')) {
    throw 'Demo binary is missing. Build it with scripts/build.ps1 first.'
}
Write-Host 'Demo only. No WireGuard installation or network configuration changes.'
Write-Host 'Open http://127.0.0.1:8787 in your browser. Press Ctrl+C to stop.'
& '.\bin\wgsimple.exe' --mode demo --listen 127.0.0.1:8787 --data-dir .data/demo
