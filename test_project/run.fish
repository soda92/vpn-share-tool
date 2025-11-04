#! /usr/bin/env fish
pushd frontend
npm run build
popd

go run main.go