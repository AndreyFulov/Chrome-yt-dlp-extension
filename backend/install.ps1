param(
    [string]$HostBinary,
    [string]$ToolsDirectory,
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$hostName = 'com.ytdlp.downloader'
$installDirectory = Join-Path $env:LOCALAPPDATA 'YtDlpChromeHost'
$registryKey = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\$hostName"
$installedHost = Join-Path $installDirectory 'host.exe'
$manifestPath = Join-Path $installDirectory "$hostName.json"

if ($Uninstall) {
    if (Test-Path -LiteralPath $registryKey) { Remove-Item -LiteralPath $registryKey }
    foreach ($file in $installedHost, $manifestPath) {
        if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file }
    }
    $tools = Join-Path $installDirectory 'tools'
    foreach ($name in 'yt-dlp.exe', 'ffmpeg.exe', 'ffprobe.exe', 'deno.exe') {
        $file = Join-Path $tools $name
        if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file }
    }
    return
}

if (-not $HostBinary -and (Test-Path -LiteralPath (Join-Path $PSScriptRoot 'host.exe') -PathType Leaf)) {
    $HostBinary = Join-Path $PSScriptRoot 'host.exe'
}
if (-not $ToolsDirectory -and (Test-Path -LiteralPath (Join-Path $PSScriptRoot 'tools') -PathType Container)) {
    $ToolsDirectory = Join-Path $PSScriptRoot 'tools'
}

if ($ToolsDirectory) {
    foreach ($name in 'yt-dlp.exe', 'ffmpeg.exe', 'ffprobe.exe', 'deno.exe') {
        if (-not (Test-Path -LiteralPath (Join-Path $ToolsDirectory $name) -PathType Leaf)) {
            throw "Missing executable in tools directory: $name"
        }
    }
}

New-Item -ItemType Directory -Path $installDirectory -Force | Out-Null
if ($HostBinary) {
    Copy-Item -LiteralPath $HostBinary -Destination $installedHost -Force
} else {
    Push-Location -LiteralPath $PSScriptRoot
    try {
        go build -trimpath -o $installedHost .
        if ($LASTEXITCODE -ne 0) { throw 'Native host build failed.' }
    } finally { Pop-Location }
}

if ($ToolsDirectory) {
    $destination = New-Item -ItemType Directory -Path (Join-Path $installDirectory 'tools') -Force
    foreach ($name in 'yt-dlp.exe', 'ffmpeg.exe', 'ffprobe.exe', 'deno.exe') {
        Copy-Item -LiteralPath (Join-Path $ToolsDirectory $name) -Destination $destination.FullName -Force
    }
}

$manifest = @{
    name = $hostName
    description = 'chrome yt-dlp extension local helper'
    path = $installedHost
    type = 'stdio'
    allowed_origins = @('chrome-extension://ifjcfdebfppkjagopedodglfheiljeap/')
} | ConvertTo-Json
[IO.File]::WriteAllText($manifestPath, $manifest, [Text.UTF8Encoding]::new($false))
New-Item -Path $registryKey -Force | Out-Null
Set-Item -LiteralPath $registryKey -Value $manifestPath
Write-Output "Installed $hostName"
