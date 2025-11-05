#! /usr/bin/env fish
pushd frontend
npm run build
popd

go build main.go