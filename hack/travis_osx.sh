#!/usr/bin/env bash
set -e

export GOPATH=$(pwd)/_gopath
_projectatomic="${GOPATH}/src/github.com/projectatomic"
mkdir -vp ${_projectatomic}
ln -vsf $(pwd) ${_projectatomic}/skopeo

cd ${_projectatomic}/skopeo
make validate-local test-unit-local binary-local
sudo make install
skopeo -v
