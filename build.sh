#!/bin/bash

if [ -z $1 ]; then
  echo "$0 <image-id>"
  exit 1
fi

docker run -v $GOPATH:/go "$1" make -C /go/src/github.com/dcos/dcos-log build
