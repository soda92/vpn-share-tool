# Build MSI Installer for VPN Share Tool
# Usage: powershell -ExecutionPolicy Bypass -File build-msi.ps1

param (
    [string]$ExePath = "",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

# 1. Locate root folder and config
$PSScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RootDir = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "../.."))

# 2. Get Version if not provided
if (-not $Version) {
    $ReleaseConfigPath = Join-Path $RootDir "Release.toml"
    if (Test-Path $ReleaseConfigPath) {
        Write-Host "Reading version from Release.toml..."
        $Content = Get-Content $ReleaseConfigPath -Raw
        # Simple regex parse for Counter
        if ($Content -match 'Counter\s*=\s*(\d+)') {
            # WiX requires major.minor.build format (integers). Suffix 'c' cannot be used in ProductVersion.
            # We map Counter to Major, and set Minor/Build to 0.
            $Version = "$($Matches[1]).0.0"
        }
    }
}
if (-not $Version) {
    $Version = "1.0.0"
}
Write-Host "Product Version: $Version"

# 3. Locate built EXE
if (-not $ExePath) {
    # Check standard build outputs
    $CandidatePaths = @(
        (Join-Path $RootDir "dist/vpn-share-tool.exe"),
        (Join-Path $RootDir "fyne-cross/bin/windows-amd64/vpn-share-tool.exe")
    )
    foreach ($Path in $CandidatePaths) {
        if (Test-Path $Path) {
            $ExePath = $Path
            break
        }
    }
}

if (-not $ExePath -or -not (Test-Path $ExePath)) {
    Write-Error "Could not locate vpn-share-tool.exe. Please build it first or specify via -ExePath."
}
$ExePath = [System.IO.Path]::GetFullPath($ExePath)
Write-Host "Source EXE: $ExePath"

# 4. Ensure WiX Toolset is available
$CandlePath = "candle.exe"
$LightPath = "light.exe"

$HasWiX = $false
try {
    $null = Get-Command $CandlePath -ErrorAction SilentlyContinue
    $null = Get-Command $LightPath -ErrorAction SilentlyContinue
    $HasWiX = $true
} catch {}

$TempWiXDir = Join-Path $PSScriptRoot "wix_bin"
if (-not $HasWiX) {
    if (Test-Path $TempWiXDir) {
        $CandlePath = Join-Path $TempWiXDir "candle.exe"
        $LightPath = Join-Path $TempWiXDir "light.exe"
        if ((Test-Path $CandlePath) -and (Test-Path $LightPath)) {
            $HasWiX = $true
        }
    }
}

if (-not $HasWiX) {
    Write-Host "WiX Toolset not found in PATH or wix_bin directory."
    Write-Host "Downloading portable WiX Toolset v3.11.2..."
    New-Item -ItemType Directory -Force -Path $TempWiXDir | Out-Null
    $ZipPath = Join-Path $TempWiXDir "wix.zip"
    $Url = "https://github.com/wixtoolset/wix3/releases/download/wix3112rtm/wix311-binaries.zip"
    
    # Use System.Net.WebClient or Invoke-WebRequest
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
    
    Write-Host "Extracting WiX Toolset..."
    Expand-Archive -Path $ZipPath -DestinationPath $TempWiXDir -Force
    Remove-Item $ZipPath -Force
    
    $CandlePath = Join-Path $TempWiXDir "candle.exe"
    $LightPath = Join-Path $TempWiXDir "light.exe"
    Write-Host "WiX Toolset downloaded successfully."
}

# 5. Compile and Link using WiX
$WxsFile = Join-Path $PSScriptRoot "vpn-share-tool.wxs"
$WixObjFile = Join-Path $PSScriptRoot "vpn-share-tool.wixobj"
$MsiOutput = Join-Path $PSScriptRoot "vpn-share-tool-$Version.msi"

Write-Host "Compiling WiX source..."
$CandleArgs = @(
    "-dVersion=$Version",
    "-dSourceExePath=$ExePath",
    "-out", $WixObjFile,
    $WxsFile
)
& $CandlePath $CandleArgs

Write-Host "Linking MSI package..."
$LightArgs = @(
    "-ext", "WixUIExtension",
    "-out", $MsiOutput,
    $WixObjFile
)
& $LightPath $LightArgs

# 6. Cleanup temp compile files
if (Test-Path $WixObjFile) { Remove-Item $WixObjFile -Force }
# Keep wix_bin for next compile runs

Write-Host "--------------------------------------------------"
Write-Host "SUCCESS! MSI built successfully."
Write-Host "Output: $MsiOutput"
Write-Host "--------------------------------------------------"
