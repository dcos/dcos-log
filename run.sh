#!/bin/bash

IMG=$(docker images journal | grep journal | awk '{print $3}')

if [ -z $IMG ] && [ -z $1 ]; then
  echo "Could not detect docker image, please set tag 'journal' or pass image ID"
  echo "$0 <image-id>"
  exit 1
fi

[ -z $IMG ] && IMG=$1

container=$(docker run -d --privileged -v $PWD:/go/src/github.com/dcos/dcos-log -p8080:8080 $IMG /sbin/init)
echo "# Using port :8080"
echo "# Run:"
echo "docker exec -it $container /bin/bash"
