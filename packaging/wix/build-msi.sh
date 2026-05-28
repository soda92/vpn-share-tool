#!/bin/bash
set -e

# Find script directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$(dirname "$(dirname "$DIR")")"

# Read version from Release.toml
VERSION="1.0.0"
if [ -f "$ROOT_DIR/Release.toml" ]; then
    COUNTER=$(grep -E '^Counter\s*=' "$ROOT_DIR/Release.toml" | head -n 1 | awk -F'=' '{print $2}' | tr -d '[:space:]')
    if [ -n "$COUNTER" ]; then
        VERSION="$COUNTER.0.0"
    fi
fi

# Find EXE
EXE_PATH=""
CANDIDATES=(
    "$ROOT_DIR/dist/vpn-share-tool.exe"
    "$ROOT_DIR/fyne-cross/bin/windows-amd64/vpn-share-tool.exe"
)
for path in "${CANDIDATES[@]}"; do
    if [ -f "$path" ]; then
        EXE_PATH="$path"
        break
    fi
done

if [ -z "$EXE_PATH" ]; then
    echo "Error: vpn-share-tool.exe not found. Please build it first."
    exit 1
fi

# Check for wixl
if ! command -v wixl &> /dev/null; then
    echo "Error: wixl not found."
    echo "Please install msitools on Arch Linux to build MSIs:"
    echo "  sudo pacman -S msitools"
    exit 1
fi

OUTPUT_MSI="$DIR/vpn-share-tool-$VERSION.msi"
echo "Building MSI with wixl..."
echo "Source EXE: $EXE_PATH"
echo "Version: $VERSION"
echo "Output: $OUTPUT_MSI"

wixl -o "$OUTPUT_MSI" -D Version="$VERSION" -D SourceExePath="$EXE_PATH" "$DIR/vpn-share-tool.wxs"

echo "✅ MSI built successfully!"
