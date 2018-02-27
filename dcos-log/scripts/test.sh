#!/usr/bin/env bash

# This script runs the tests for the dcos-log go code.  In order to provide
# an environment where systemd/journald is running, it first starts up
# a container running /sbin/init.  It then execs into that container to run
# the go test suite. Before the script exits, it will destroy the container.

set -e # exit on failure
set -x # print each command

cleanup() {
	echo "Cleaning up the container..."
	docker rm -f ${CONTAINER_NAME} >/dev/null
}
trap cleanup EXIT

# print docker options
docker exec --help

echo "Starting container that is running systemd and journald..."
echo "current dir is ${CURRENT_DIR}"
docker run \
	-d \
	-v ${CURRENT_DIR}:${PKG_DIR}/${BINARY_NAME} \
	--privileged \
	--rm \
	--name ${CONTAINER_NAME}  \
	${IMAGE_NAME} \
	/sbin/init >/dev/null

echo "Running tests against that container..."
docker exec \
	${CONTAINER_NAME} \
	bash -c 'cd /go/src/github.com/dcos/dcos-log &&
		/usr/local/go/bin/go test -race -v $(go list ./...|grep -v vendor)'
		#/usr/local/go/bin/go test -race -cover -test.v $(go list ./...|grep -v vendor)'

