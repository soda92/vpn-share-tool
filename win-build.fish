#! /usr/bin/env fish
set -x GOOS windows
set -x GOARCH amd64
set -x CGO_ENABLED 1
set -x CC /usr/bin/x86_64-w64-mingw32-gcc
set -x CGO_LDFLAGS "-static-libgcc -static"
go build -ldflags="-H windowsgui" -o vpn-share-tool.exe
