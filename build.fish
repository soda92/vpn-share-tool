#! /usr/bin/env fish
pushd core/frontend
npm run build
popd

go build