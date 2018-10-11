IMAGE_NAME=dcos-log-dev
CONTAINER_NAME=$(IMAGE_NAME)-test
BUILD_DIR=build
CURRENT_DIR=$(shell pwd)
BINARY_NAME=dcos-log
PKG_DIR=/go/src/github.com/dcos
DCOS_LOG_PKG_DIR=$(PKG_DIR)/dcos-log

all: lint vet test build

lint: docker
	@echo "+$@"
	docker run \
		-v $(CURRENT_DIR):$(PKG_DIR)/$(BINARY_NAME) \
		-w $(DCOS_LOG_PKG_DIR) \
		--privileged \
		--rm \
		$(IMAGE_NAME) \
		go list ./... | grep -v /vendor/ | xargs -L1 golint -set_exit_status

test: docker lint vet
	@echo "+$@"
	docker run \
		-v $(CURRENT_DIR):$(PKG_DIR)/$(BINARY_NAME) \
		-w $(DCOS_LOG_PKG_DIR) \
		--privileged \
		--rm \
		$(IMAGE_NAME) \
		go test ./... -race -cover -timeout 5m

build: docker
	@echo "+$@"
	mkdir -p $(BUILD_DIR)
	docker run \
		-v $(CURRENT_DIR):$(PKG_DIR)/$(BINARY_NAME) \
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
		-v $(CURRENT_DIR):$(PKG_DIR)/$(BINARY_NAME) \
		-w $(DCOS_LOG_PKG_DIR) \
		--privileged \
		--rm \
		$(IMAGE_NAME) \
		go vet ./...

.PHONY: docker
docker:
	docker build -t $(IMAGE_NAME) .

