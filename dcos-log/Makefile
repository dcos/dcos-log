all: lint vet test build

install:
	@echo "+$@"
	go install

gofmt:
	@echo "+$@"
	go fmt ./...

lint:
	@echo "+$@"
	go get -u github.com/golang/lint/golint
	golint -set_exit_status ./...

test: lint vet
	@echo "+$@"
	go test -race -cover -test.v ./...

build:
	@echo "+$@"
	go build

clean:
	@echo "+$@"
	rm -f dcos-log

vet:
	@echo "+$@"
	go vet ./...

