# Top level docker container used to build go projects.

# Build all projects:
# docker run -v $GOPATH:/go <contaier-id> make -C /go/src/github.com/dcos/dcos-log

FROM golang:1.7

RUN apt-get update && apt-get install -y \
    # used by dcos-log
    libsystemd-dev

ENV PATH /go/bin:/usr/local/go/bin:$PATH
