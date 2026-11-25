param(
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '../dist'),
    [string]$ToolBundles
)

$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot
$output = New-Item -ItemType Directory -Path $OutputDirectory -Force
$staging = New-Item -ItemType Directory -Path (Join-Path $output.FullName ('.staging-' + [guid]::NewGuid()))
$previous = @{ GOOS = $env:GOOS; GOARCH = $env:GOARCH; CGO_ENABLED = $env:CGO_ENABLED }
$archives = @()

try {
    $env:CGO_ENABLED = '0'
    foreach ($target in @(
        @{ OS = 'windows'; Arch = 'amd64'; Host = 'host.exe'; Installer = 'install.ps1' },
        @{ OS = 'darwin'; Arch = 'amd64'; Host = 'host'; Installer = 'install-macos.sh' },
        @{ OS = 'darwin'; Arch = 'arm64'; Host = 'host'; Installer = 'install-macos.sh' }
    )) {
        $platform = "$($target.OS)-$($target.Arch)"
        $package = Join-Path $staging.FullName "ytdlp-chrome-$platform"
        $backend = New-Item -ItemType Directory -Path (Join-Path $package 'backend') -Force
        $bundle = if ($ToolBundles) { Join-Path $ToolBundles $platform }
        $toolNames = 'yt-dlp', 'ffmpeg', 'ffprobe', 'deno'
        if ($target.OS -eq 'windows') { $toolNames = $toolNames | ForEach-Object { "$_.exe" } }
        if ($bundle) {
            foreach ($name in $toolNames) {
                if (-not (Test-Path -LiteralPath (Join-Path $bundle $name) -PathType Leaf)) {
                    throw "Missing $name in $bundle"
                }
            }
            $tools = New-Item -ItemType Directory -Path (Join-Path $backend.FullName 'tools') -Force
            foreach ($name in $toolNames) {
                Copy-Item -LiteralPath (Join-Path $bundle $name) -Destination $tools.FullName -Force
            }
        }
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        Push-Location -LiteralPath $PSScriptRoot
        try {
            go build -trimpath -o (Join-Path $backend.FullName $target.Host) .
            if ($LASTEXITCODE -ne 0) { throw "Build failed for $platform" }
        } finally { Pop-Location }
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $target.Installer) -Destination $backend.FullName -Force
        Copy-Item -LiteralPath (Join-Path $root 'frontend') -Destination $package -Recurse -Force
        Copy-Item -LiteralPath (Join-Path $root 'README.md') -Destination $package -Force
        Copy-Item -LiteralPath (Join-Path $root 'LICENSE') -Destination $package -Force
        $zip = Join-Path $output.FullName "ytdlp-chrome-$platform.zip"
        Compress-Archive -LiteralPath $package -DestinationPath $zip -Force
        $archives += Get-Item -LiteralPath $zip
        Write-Output $zip
    }
    $checksum = Join-Path $output.FullName 'SHA256SUMS.txt'
    $hashLines = $archives | Sort-Object Name | ForEach-Object {
        $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $($_.Name)"
    }
    $newLine = [char]10
    [IO.File]::WriteAllText($checksum, ($hashLines -join $newLine) + $newLine)
    Write-Output $checksum
} finally {
    foreach ($variable in $previous.Keys) {
        [Environment]::SetEnvironmentVariable($variable, $previous[$variable], 'Process')
    }
    $resolvedStaging = (Resolve-Path -LiteralPath $staging.FullName).ProviderPath
    if ((Split-Path -Parent $resolvedStaging) -eq $output.FullName -and
        (Split-Path -Leaf $resolvedStaging) -like '.staging-*') {
        Remove-Item -LiteralPath $resolvedStaging -Recurse -Force
    }
}
