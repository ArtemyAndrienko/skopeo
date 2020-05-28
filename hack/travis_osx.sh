#!/usr/bin/env bash
set -e

export GOPATH=$(pwd)/_gopath
export PATH=$GOPATH/bin:$PATH

_containers="${GOPATH}/src/github.com/containers"
mkdir -vp ${_containers}
ln -vsf $(pwd) ${_containers}/skopeo

go version
GO111MODULE=off go get -u github.com/cpuguy83/go-md2man golang.org/x/lint/golint

cd ${_containers}/skopeo
make validate-local test-unit-local bin/skopeo
sudo make install
skopeo -v
