IMAGE_NAME=dcos-log-dev
CONTAINER_NAME=$(IMAGE_NAME)-test
BUILD_DIR=build
CURRENT_DIR=$(shell pwd)
BINARY_NAME=dcos-log
PKG_DIR=/go/src/github.com/dcos
DCOS_LOG_PKG_DIR=$(PKG_DIR)/dcos-log/dcos-log

all: lint vet test build

install: docker
	@echo "+$@"
	go install

gofmt:
	@echo "+$@"
	go fmt ./...

lint: docker
	@echo "+$@"
	golint -set_exit_status ./...

test: docker lint vet
	@echo "+$@"
	@bash -c ' \
		CURRENT_DIR=$(CURRENT_DIR) \
		PKG_DIR=$(PKG_DIR) \
		BINARY_NAME=$(BINARY_NAME) \
		IMAGE_NAME=$(IMAGE_NAME) \
		CONTAINER_NAME=$(CONTAINER_NAME) \
		scripts/test.sh'

build: docker
	@echo "+$@"
	mkdir -p $(BUILD_DIR)
	docker run \
		-v $(CURRENT_DIR)/..:$(PKG_DIR)/$(BINARY_NAME) \
		-w $(DCOS_LOG_PKG_DIR) \
		--privileged \
		--rm \
		$(IMAGE_NAME) \
		go build -v -o $(BUILD_DIR)/$(BINARY_NAME)

clean:
	@echo "+$@"
	rm -rf $(BUILD_DIR)

vet: docker
	@echo "+$@"
	docker run \
		-v $(CURRENT_DIR)/..:$(PKG_DIR)/$(BINARY_NAME) \
		-w $(DCOS_LOG_PKG_DIR) \
		--privileged \
		--rm \
		$(IMAGE_NAME) \
		go vet ./...

.PHONY: docker
docker:
	docker build -t $(IMAGE_NAME) .

