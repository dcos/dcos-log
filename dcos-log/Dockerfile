FROM golang:1.7

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

RUN apt-get update && apt-get install -y \
    libsystemd-dev \
    init

RUN go get -u golang.org/x/lint/golint

