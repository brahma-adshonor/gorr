#! /bin/bash

set -e

CURPATH=`pwd`
export GOPATH="$GOPATH:$CURPATH"

#go test grpc_hook_test.go
go test -gcflags=all='-l' -o t1 -cover 

cd tool
go test -gcflags=all='-l' -o t2 -cover 

cd diff
go test

cd ../dbtool
go test

