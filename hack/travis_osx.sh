#!/usr/bin/env bash
set -e

export GOPATH=$(pwd)/_gopath
export PATH=$GOPATH/bin:$PATH

_projectatomic="${GOPATH}/src/github.com/projectatomic"
mkdir -vp ${_projectatomic}
ln -vsf $(pwd) ${_projectatomic}/skopeo

go get -u github.com/cpuguy83/go-md2man github.com/golang/lint/golint

cd ${_projectatomic}/skopeo
make validate-local test-unit-local binary-local
sudo make install
skopeo -v
