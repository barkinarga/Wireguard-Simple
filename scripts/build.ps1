param([switch]$All)
$ErrorActionPreference = 'Stop'
$previousBuildEnv = @{}
foreach ($buildEnvName in @('GOOS','GOARCH','CGO_ENABLED','GOPATH','GOCACHE')) {
    $previousBuildEnv[$buildEnvName] = [Environment]::GetEnvironmentVariable($buildEnvName, 'Process')
}
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    $goExecutable = if (Test-Path -LiteralPath '.tools\go\bin\go.exe') { (Resolve-Path -LiteralPath '.tools\go\bin\go.exe').Path } else { 'go' }
    if (Test-Path -LiteralPath '.tools\gopath') { $env:GOPATH = Join-Path (Get-Location) '.tools\gopath' }
    if (Test-Path -LiteralPath '.tools\gocache') { $env:GOCACHE = Join-Path (Get-Location) '.tools\gocache' }
    npm.cmd run check
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript validation failed' }
    npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'UI build failed' }
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    & $goExecutable build -trimpath -ldflags='-s -w' -o bin/wgsimple.exe ./cmd/wgsimple
    if ($LASTEXITCODE -ne 0) { throw 'Windows build failed' }
    if ($All) {
        $env:GOOS = 'linux'
        foreach ($targetArch in @('amd64','arm64')) {
            $env:GOARCH = $targetArch
            & $goExecutable build -trimpath -ldflags='-s -w' -o "bin/wgsimple-linux-$targetArch" ./cmd/wgsimple
            if ($LASTEXITCODE -ne 0) { throw "Linux $targetArch build failed" }
        }
        node scripts/package-linux.mjs
        if ($LASTEXITCODE -ne 0) { throw 'Linux installer packaging failed' }
    }
    Get-ChildItem -LiteralPath 'bin' | Select-Object Name,Length
} finally {
    foreach ($buildEnvName in $previousBuildEnv.Keys) {
        [Environment]::SetEnvironmentVariable($buildEnvName, $previousBuildEnv[$buildEnvName], 'Process')
    }
    Pop-Location
}
