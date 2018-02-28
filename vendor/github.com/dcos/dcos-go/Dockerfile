FROM golang:1.8

# libltdl7 is needed to run the Docker CLI
RUN apt-get update \
  && apt-get install -y libltdl7 \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /go/src/github.com/dcos/dcos-go
