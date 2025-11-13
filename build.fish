#! /usr/bin/env fish
rm -r core/frontend/dist
pushd core/frontend
npm run build
popd

go build