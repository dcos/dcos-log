FROM golang:1.11

RUN apt-get update && apt-get install -y \
    libsystemd-dev \
    init

RUN go get -u golang.org/x/lint/golint