#! /usr/bin/env fish
pushd core/frontend
npm run build
popd

fyne-cross windows -arch amd64 --app-id vpn.share.tool