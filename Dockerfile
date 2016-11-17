FROM golang:1.7

RUN apt-get update && apt-get install -y \
    libsystemd-dev

ENV PATH /go/bin:/usr/local/go/bin:$PATH
