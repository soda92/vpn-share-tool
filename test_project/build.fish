#! /usr/bin/env fish
rm -r frontend/dist
pushd frontend
npm run build
popd

go build main.go