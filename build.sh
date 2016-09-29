#!/bin/bash

IMG=$(docker images journal | grep journal | awk '{print $3}')

if [ -z $IMG ] && [ -z $1 ]; then
  echo "Could not detect docker image, please set tag 'journal' or pass image ID"
  echo "$0 <image-id>"
  exit 1
fi

[ -z $IMG ] && IMG=$1

docker run -v $PWD:/go/src/github.com/dcos/dcos-log $IMG make -C /go/src/github.com/dcos/dcos-log
