param([ValidateRange(1024,65535)][int]$Port = 8787)
$ErrorActionPreference = 'Stop'
Push-Location -LiteralPath $PSScriptRoot
$previousPreviewEnv = @{}
foreach ($previewEnvName in @('GOPATH','GOCACHE','CGO_ENABLED','GOOS','GOARCH')) {
    $previousPreviewEnv[$previewEnvName] = [Environment]::GetEnvironmentVariable($previewEnvName, 'Process')
}
try {
    npm.cmd run check
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript validation failed' }
    npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'UI build failed' }
    $previewGo = if (Test-Path -LiteralPath '.tools\go\bin\go.exe') { '.tools\go\bin\go.exe' } else { 'go' }
    New-Item -ItemType Directory -Force -Path '.tools' | Out-Null
    if (Test-Path -LiteralPath '.tools\gopath') { $env:GOPATH = Join-Path (Get-Location) '.tools\gopath' }
    if (Test-Path -LiteralPath '.tools\gocache') { $env:GOCACHE = Join-Path (Get-Location) '.tools\gocache' }
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    & $previewGo build -o '.tools\wgsimple-design-preview.exe' ./cmd/wgsimple
    if ($LASTEXITCODE -ne 0) { throw 'Preview build failed' }
    Write-Host "Original: http://127.0.0.1:$Port/?design=v1"
    Write-Host "V2:       http://127.0.0.1:$Port/"
    Write-Host 'Demo only. Press Ctrl+C to stop.'
    & '.\.tools\wgsimple-design-preview.exe' --mode demo --listen "127.0.0.1:$Port" --data-dir '.data/design-preview-v2'
    if ($LASTEXITCODE -ne 0) { throw 'Preview stopped with an error. Check whether the port is already in use.' }
} finally {
    foreach ($previewEnvName in $previousPreviewEnv.Keys) {
        [Environment]::SetEnvironmentVariable($previewEnvName, $previousPreviewEnv[$previewEnvName], 'Process')
    }
    Pop-Location
}
