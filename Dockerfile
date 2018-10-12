FROM golang:1.11

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN apt-get update && apt-get install -y \
    libsystemd-dev \
    init
